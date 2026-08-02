package kannon

import (
	"context"
	"time"
)

// HTMLSender is the part of the Kannon client that MailSender needs: sending
// an HTML email with options. *Kannon satisfies it.
type HTMLSender interface {
	SendEmail(ctx context.Context, to []Recipient, subject, body string, opts ...SendOption) (MessageResult, error)
}

// noTracking opts out of open and link tracking: a plain mail client sends
// mail, it does not measure it.
var noTracking = TrackingPolicy{Opens: TrackingModeOff, Links: TrackingModeOff}

// MailSender is a simplified mail client for sending emails without
// templating.
//
// It wraps the Kannon client with a traditional mail API that accepts plain
// HTML content and supports To, CC, and BCC recipients.
//
// Mail sent this way is never tracked: opens and links are both forced to
// off, and no List-Unsubscribe header is emitted. Use the Kannon client
// directly for campaign mail, where those are decisions to make.
type MailSender struct {
	client HTMLSender
}

// NewMailSender creates a new MailSender on top of the given client,
// typically a *Kannon.
func NewMailSender(client HTMLSender) *MailSender {
	return &MailSender{client: client}
}

// SendParams are the parameters for sending an email through MailSender.
type SendParams struct {
	// To lists the primary recipients. Required: at least one is needed.
	To []string
	// CC lists the carbon copy recipients, visible to all recipients.
	CC []string
	// BCC lists the blind carbon copy recipients. They receive the email but
	// are not visible to other recipients.
	BCC []string
	// Subject is the email subject line.
	Subject string
	// Content is the email body as HTML.
	Content string
	// Attachments are the files attached to the email.
	Attachments []Attachment
	// ScheduledTime schedules the email for future delivery. When nil, the
	// email is sent immediately.
	ScheduledTime *time.Time
}

// SendResult is the result returned after sending an email through
// MailSender.
type SendResult struct {
	// MessageID is the unique identifier of the sent message.
	MessageID string
	// AcceptedCount is how many recipients were accepted and are queued for
	// delivery.
	AcceptedCount int
	// RejectedCount is how many recipients were refused at intake.
	RejectedCount int
	// RejectedRecipients lists every recipient refused at intake, with the
	// reason for the refusal.
	RejectedRecipients []RejectedRecipient
	// ScheduledTime is the scheduled delivery time.
	ScheduledTime time.Time
}

// Send sends an email.
//
//   - CC recipients are visible in the email headers to all recipients.
//   - BCC recipients receive the email but never appear in the headers.
//   - Open and link tracking are always disabled.
//   - Recipients refused at intake are reported in RejectedRecipients
//     instead of failing the send.
func (m *MailSender) Send(ctx context.Context, params SendParams) (SendResult, error) {
	recipients := make([]Recipient, 0, len(params.To)+len(params.CC)+len(params.BCC))
	for _, list := range [][]string{params.To, params.CC, params.BCC} {
		for _, email := range list {
			recipients = append(recipients, Recipient{Email: email})
		}
	}

	headers := Headers{To: params.To}
	if len(params.CC) > 0 {
		headers.CC = params.CC
	}

	opts := []SendOption{
		WithAttachments(params.Attachments),
		WithHeaders(headers),
		WithTracking(noTracking),
	}
	if params.ScheduledTime != nil {
		opts = append(opts, WithScheduledTime(*params.ScheduledTime))
	}

	result, err := m.client.SendEmail(ctx, recipients, params.Subject, params.Content, opts...)
	if err != nil {
		return SendResult{}, err
	}

	return SendResult{
		MessageID:          result.MessageID,
		AcceptedCount:      result.AcceptedCount,
		RejectedCount:      result.RejectedCount,
		RejectedRecipients: result.RejectedRecipients,
		ScheduledTime:      result.ScheduledTime,
	}, nil
}
