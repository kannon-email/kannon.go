// Package kannon is the official Go client for Kannon, the cloud-native
// email sender. It sends through the Kannon REST API v1.
package kannon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the REST API v1 of the hosted Kannon at kannon.email.
const DefaultBaseURL = "https://api.kannon.email/v1"

// maxResponseBytes bounds how much of a response body is read, so a
// misbehaving proxy cannot exhaust memory.
const maxResponseBytes = 1 << 20

// Kannon sends emails for one Domain with one API Key.
type Kannon struct {
	domain     string
	key        string
	sender     Sender
	baseURL    string
	httpClient *http.Client
}

// ClientOption customizes a Kannon client.
type ClientOption func(*Kannon)

// WithBaseURL points the client at a Kannon REST API other than the hosted
// one, including the version prefix (for example "https://kannon.example.com/v1").
func WithBaseURL(baseURL string) ClientOption {
	return func(k *Kannon) {
		k.baseURL = baseURL
	}
}

// WithHTTPClient sets the HTTP client used for every request. It defaults to
// http.DefaultClient.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(k *Kannon) {
		k.httpClient = client
	}
}

// NewKannon creates a client that sends as sender on domain, authenticated by
// the Domain API Key key.
func NewKannon(domain, key string, sender Sender, opts ...ClientOption) *Kannon {
	k := &Kannon{
		domain:     domain,
		key:        key,
		sender:     sender,
		baseURL:    DefaultBaseURL,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(k)
	}
	k.baseURL = strings.TrimRight(k.baseURL, "/")
	return k
}

// SendEmail sends an HTML email to the given recipients. The send can be
// customized with SendOption values such as WithTracking, WithHeaders or
// WithScheduledTime.
func (k *Kannon) SendEmail(ctx context.Context, to []Recipient, subject, body string, opts ...SendOption) (MessageResult, error) {
	return k.createBatch(ctx, to, subject, wireContent{Type: "html", HTML: body}, opts)
}

// SendTemplate sends a template email to the given recipients. The send can
// be customized with SendOption values such as WithTracking, WithHeaders or
// WithScheduledTime.
func (k *Kannon) SendTemplate(ctx context.Context, to []Recipient, subject, templateID string, opts ...SendOption) (MessageResult, error) {
	return k.createBatch(ctx, to, subject, wireContent{Type: "template", TemplateID: templateID}, opts)
}

func (k *Kannon) createBatch(ctx context.Context, to []Recipient, subject string, content wireContent, opts []SendOption) (MessageResult, error) {
	options := applySendOptions(opts)

	idempotencyKey := options.idempotencyKey
	if idempotencyKey == "" {
		var err error
		if idempotencyKey, err = newIdempotencyKey(); err != nil {
			return MessageResult{}, err
		}
	}

	payload, err := json.Marshal(newWireBatch(k.sender, to, subject, content, options))
	if err != nil {
		return MessageResult{}, fmt.Errorf("kannon: encode batch: %w", err)
	}

	endpoint := k.baseURL + "/domains/" + url.PathEscape(k.domain) + "/batches"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return MessageResult{}, fmt.Errorf("kannon: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+k.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, application/problem+json")
	req.Header.Set("Idempotency-Key", idempotencyKey)

	res, err := k.httpClient.Do(req)
	if err != nil {
		return MessageResult{}, fmt.Errorf("kannon: send batch: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return MessageResult{}, fmt.Errorf("kannon: read response: %w", err)
	}

	switch res.StatusCode {
	case http.StatusCreated, http.StatusOK:
		var created wireBatchCreated
		if err := json.Unmarshal(body, &created); err != nil {
			return MessageResult{}, fmt.Errorf("kannon: decode response: %w", err)
		}
		return created.toResult(to, res.StatusCode == http.StatusOK), nil
	default:
		return MessageResult{}, newAPIError(res, body)
	}
}

// MessageResult is the intake receipt of a Batch.
type MessageResult struct {
	// BatchID identifies the created Batch.
	BatchID string
	// ScheduledTime is when delivery starts: the creation time for a batch
	// sent immediately.
	ScheduledTime time.Time
	// AcceptedCount is how many recipients were accepted and are queued for
	// delivery.
	AcceptedCount int
	// RejectedCount is how many recipients were refused at intake.
	RejectedCount int
	// RejectedRecipients lists every recipient refused at intake, so you can
	// reconcile what was actually queued without polling stats. It is empty
	// on a replay, whose RejectedCount still holds the original count.
	//
	// A send in which every recipient is refused does not create a Batch: it
	// fails with an *APIError of code CodeNoRecipientsAccepted.
	RejectedRecipients []RejectedRecipient
	// Replayed reports that the idempotency key had already created this
	// Batch, so no new Batch or Delivery was created.
	Replayed bool
}

// RejectionReason is why a recipient was refused at intake.
//
// It is a stable, machine-readable token, safe to branch on. Treat an
// unrecognised value as a refusal of unknown cause: the set grows as new
// causes are added.
type RejectionReason string

const (
	// RejectionReasonInvalidEmail means the address is empty or otherwise
	// unusable.
	RejectionReasonInvalidEmail RejectionReason = "invalid_email"
	// RejectionReasonDuplicateRecipient means the address already appears
	// earlier in the same batch.
	RejectionReasonDuplicateRecipient RejectionReason = "duplicate_recipient"
	// RejectionReasonTrackingAboveCeiling means the recipient's tracking
	// policy asks for more than its domain allows; consent may narrow what is
	// collected, never widen it.
	RejectionReasonTrackingAboveCeiling RejectionReason = "tracking_above_ceiling"
	// RejectionReasonUnsupportedTrackingMode means the recipient stated a
	// tracking mode the server will not act on: the reserved
	// TrackingModePseudonymous, or a value from a newer schema.
	RejectionReasonUnsupportedTrackingMode RejectionReason = "unsupported_tracking_mode"
	// RejectionReasonUnsubscribeURLUnresolved means the recipient's fields
	// leave a placeholder in the one-click unsubscribe URL template
	// unresolved.
	RejectionReasonUnsubscribeURLUnresolved RejectionReason = "unsubscribe_url_unresolved"
	// RejectionReasonUnresolvedPlaceholder means the recipient's fields leave
	// a placeholder in the subject or content unresolved.
	RejectionReasonUnresolvedPlaceholder RejectionReason = "unresolved_placeholder"
	// RejectionReasonReservedField means the recipient's fields override a
	// reserved field, such as email.
	RejectionReasonReservedField RejectionReason = "reserved_field"
	// RejectionReasonMessageTooLarge means the personalised message for this
	// recipient exceeds the size the server accepts.
	RejectionReasonMessageTooLarge RejectionReason = "message_too_large"
)

// RejectedRecipient is one recipient refused at intake: no delivery was
// created for it.
type RejectedRecipient struct {
	// Index is the zero-based position of the recipient in the request.
	Index  int
	Email  string
	Reason RejectionReason
}

// Sender is the From of every email the client sends.
type Sender struct {
	Email string
	// Alias is the display name shown beside Email.
	Alias string
}

type Recipient struct {
	Email string
	// Fields personalise the subject and content for this recipient. email
	// is reserved for the recipient's address: setting it rejects the
	// recipient with RejectionReasonReservedField.
	Fields map[string]string
	// Tracking is the consent this recipient has given, as held in your own
	// CRM.
	//
	// Nil states nothing and imposes no restriction of its own. It may only
	// narrow what the batch and the domain already allow: a recipient asking
	// for more than its domain permits is rejected on its own, with reason
	// RejectionReasonTrackingAboveCeiling, while the rest of the batch
	// proceeds.
	Tracking *TrackingPolicy
}

type Fields map[string]string
