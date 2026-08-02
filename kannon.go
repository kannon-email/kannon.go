package kannon

import (
	"context"
	"encoding/base64"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/kannon-email/kannon.go/internal/proto/kannon/mailer/apiv1"
	k "github.com/kannon-email/kannon.go/internal/proto/kannon/mailer/apiv1/apiv1connect"
	"github.com/kannon-email/kannon.go/internal/proto/kannon/mailer/types"
	trackingtypes "github.com/kannon-email/kannon.go/internal/proto/kannon/tracking/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Kannon struct {
	domain string
	key    string

	sender Sender

	cli k.MailerClient
}

func NewKannon(domain, key string, sender Sender, host string) *Kannon {
	cli := k.NewMailerClient(http.DefaultClient, host, connect.WithGRPC())

	return &Kannon{
		domain: domain,
		key:    key,

		sender: sender,
		cli:    cli,
	}
}

// SendEmail sends an HTML email to the given recipients. The send can be
// customized with SendOption values such as WithTracking, WithHeaders or
// WithScheduledTime.
func (k *Kannon) SendEmail(ctx context.Context, to []Recipient, subject, body string, opts ...SendOption) (MessageResult, error) {
	common := k.buildSendRequest(to, subject, opts)

	req := connect.NewRequest(&apiv1.SendHTMLReq{
		Sender:              common.sender,
		Subject:             common.subject,
		Html:                body,
		Recipients:          common.recipients,
		Attachments:         common.attachments,
		GlobalFields:        common.globalFields,
		Headers:             common.headers,
		OneClickUnsubscribe: common.oneClickUnsubscribe,
		ScheduledTime:       common.scheduledTime,
		Tracking:            common.tracking,
	})

	req.Header().Set("Authorization", k.getAuthHeader())

	res, err := k.cli.SendHTML(ctx, req)
	if err != nil {
		return MessageResult{}, err
	}

	return parseResult(res.Msg), nil
}

// SendTemplate sends a template email to the given recipients. The send can
// be customized with SendOption values such as WithTracking, WithHeaders or
// WithScheduledTime.
func (k *Kannon) SendTemplate(ctx context.Context, to []Recipient, subject, templateID string, opts ...SendOption) (MessageResult, error) {
	common := k.buildSendRequest(to, subject, opts)

	req := connect.NewRequest(&apiv1.SendTemplateReq{
		Sender:              common.sender,
		Subject:             common.subject,
		TemplateId:          templateID,
		Recipients:          common.recipients,
		Attachments:         common.attachments,
		GlobalFields:        common.globalFields,
		Headers:             common.headers,
		OneClickUnsubscribe: common.oneClickUnsubscribe,
		ScheduledTime:       common.scheduledTime,
		Tracking:            common.tracking,
	})

	req.Header().Set("Authorization", k.getAuthHeader())

	res, err := k.cli.SendTemplate(ctx, req)
	if err != nil {
		return MessageResult{}, err
	}

	return parseResult(res.Msg), nil
}

// sendRequestFields carries the proto fields shared by SendHTMLReq and
// SendTemplateReq, so both send methods build them the same way.
type sendRequestFields struct {
	sender              *types.Sender
	subject             string
	recipients          []*types.Recipient
	attachments         []*apiv1.Attachment
	globalFields        map[string]string
	headers             *types.Headers
	oneClickUnsubscribe *types.OneClickUnsubscribe
	scheduledTime       *timestamppb.Timestamp
	tracking            *trackingtypes.TrackingPolicy
}

// buildSendRequest builds the fields shared by SendHTMLReq and
// SendTemplateReq. Optional messages are omitted (nil) when the matching
// option is absent: in particular an absent scheduled time means "send now"
// server-side.
func (k *Kannon) buildSendRequest(to []Recipient, subject string, opts []SendOption) sendRequestFields {
	options := applySendOptions(opts)

	fields := sendRequestFields{
		sender:       k.sender.toProto(),
		subject:      subject,
		recipients:   mapRecipients(to),
		attachments:  mapAttachments(options.attachments),
		globalFields: options.globalFields,
		tracking:     options.tracking.toProto(),
	}

	if fields.globalFields == nil {
		fields.globalFields = map[string]string{}
	}

	if options.headers != nil {
		fields.headers = &types.Headers{
			To: emptyIfNil(options.headers.To),
			Cc: emptyIfNil(options.headers.CC),
		}
	}

	if options.oneClickUnsubscribe != nil {
		fields.oneClickUnsubscribe = &types.OneClickUnsubscribe{
			UrlTemplate: options.oneClickUnsubscribe.URLTemplate,
		}
	}

	if options.scheduledTime != nil {
		fields.scheduledTime = timestamppb.New(*options.scheduledTime)
	}

	return fields
}

func (k *Kannon) getAuthHeader() string {
	token := base64.StdEncoding.EncodeToString([]byte(k.domain + ":" + k.key))
	return "Basic " + token
}

func mapRecipients(recipients []Recipient) []*types.Recipient {
	res := make([]*types.Recipient, 0, len(recipients))
	for _, r := range recipients {
		fields := r.Fields
		if fields == nil {
			fields = map[string]string{}
		}
		res = append(res, &types.Recipient{
			Email:    r.Email,
			Fields:   fields,
			Tracking: r.Tracking.toProto(),
		})
	}
	return res
}

func mapAttachments(attachments []Attachment) []*apiv1.Attachment {
	res := make([]*apiv1.Attachment, 0, len(attachments))
	for _, a := range attachments {
		res = append(res, &apiv1.Attachment{
			Filename: a.Filename,
			Content:  a.Content,
		})
	}
	return res
}

func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func parseResult(res *apiv1.SendRes) MessageResult {
	scheduledTime := time.Now()
	if res.ScheduledTime != nil {
		scheduledTime = res.ScheduledTime.AsTime()
	}

	rejected := make([]RejectedRecipient, 0, len(res.RejectedRecipients))
	for _, r := range res.RejectedRecipients {
		rejected = append(rejected, RejectedRecipient{
			Email:  r.Email,
			Reason: RejectionReason(r.Reason),
		})
	}

	return MessageResult{
		MessageID:          res.MessageId,
		TemplateID:         res.TemplateId,
		ScheduledTime:      scheduledTime,
		AcceptedCount:      int(res.AcceptedCount),
		RejectedCount:      int(res.RejectedCount),
		RejectedRecipients: rejected,
	}
}

type MessageResult struct {
	MessageID     string
	TemplateID    string
	ScheduledTime time.Time
	// AcceptedCount is how many recipients were accepted and are queued for
	// delivery.
	AcceptedCount int
	// RejectedCount is how many recipients were refused at intake. It equals
	// the length of RejectedRecipients, and is zero for a send in which
	// nothing was refused.
	RejectedCount int
	// RejectedRecipients lists every recipient refused at intake, so you can
	// reconcile what was actually queued without polling stats. A send in
	// which every recipient is refused still succeeds, with AcceptedCount
	// zero and every refusal listed.
	RejectedRecipients []RejectedRecipient
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
)

// RejectedRecipient is one recipient refused at intake: no delivery was
// created for it.
type RejectedRecipient struct {
	Email  string
	Reason RejectionReason
}

type Sender struct {
	Email string
	Alias string
}

func (s Sender) toProto() *types.Sender {
	return &types.Sender{
		Email: s.Email,
		Alias: s.Alias,
	}
}

type Recipient struct {
	Email  string
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
