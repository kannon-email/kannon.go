package kannon

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

type wireBatch struct {
	From                wireMailbox       `json:"from"`
	Subject             string            `json:"subject"`
	Content             wireContent       `json:"content"`
	Recipients          []wireRecipient   `json:"recipients"`
	ScheduledAt         *time.Time        `json:"scheduled_at,omitempty"`
	GlobalFields        map[string]string `json:"global_fields,omitempty"`
	Attachments         []wireAttachment  `json:"attachments,omitempty"`
	Headers             *wireHeaders      `json:"headers,omitempty"`
	Tracking            *wireTracking     `json:"tracking,omitempty"`
	OneClickUnsubscribe *wireUnsubscribe  `json:"one_click_unsubscribe,omitempty"`
	ReplyTo             *wireMailbox      `json:"reply_to,omitempty"`
}

type wireMailbox struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type wireContent struct {
	Type       string `json:"type"`
	HTML       string `json:"html,omitempty"`
	TemplateID string `json:"template_id,omitempty"`
}

type wireRecipient struct {
	Email    string            `json:"email"`
	Fields   map[string]string `json:"fields,omitempty"`
	Tracking *wireTracking     `json:"tracking,omitempty"`
}

type wireTracking struct {
	Opens TrackingMode `json:"opens,omitempty"`
	Links TrackingMode `json:"links,omitempty"`
}

type wireAttachment struct {
	Filename      string `json:"filename"`
	ContentBase64 string `json:"content_base64"`
}

type wireHeaders struct {
	To []string `json:"to,omitempty"`
	CC []string `json:"cc,omitempty"`
}

type wireUnsubscribe struct {
	URLTemplate string `json:"url_template"`
}

// wireBatchCreated decodes both the 201 creation receipt and the 200 replay,
// which lacks rejected_recipients.
type wireBatchCreated struct {
	ID                 string          `json:"id"`
	ScheduledAt        time.Time       `json:"scheduled_at"`
	AcceptedCount      int             `json:"accepted_count"`
	RejectedCount      int             `json:"rejected_count"`
	RejectedRecipients []wireRejection `json:"rejected_recipients"`
}

type wireRejection struct {
	Index int    `json:"index"`
	Email string `json:"email"`
	Code  string `json:"code"`
}

func newWireBatch(sender Sender, to []Recipient, subject string, content wireContent, options sendOptions) wireBatch {
	batch := wireBatch{
		From:         wireMailbox{Email: sender.Email, Name: sender.Alias},
		Subject:      subject,
		Content:      content,
		Recipients:   make([]wireRecipient, 0, len(to)),
		ScheduledAt:  options.scheduledTime,
		GlobalFields: options.globalFields,
		Tracking:     options.tracking.toWire(),
		ReplyTo:      options.replyTo,
	}

	for _, r := range to {
		batch.Recipients = append(batch.Recipients, wireRecipient{
			Email:    r.Email,
			Fields:   r.Fields,
			Tracking: r.Tracking.toWire(),
		})
	}

	for _, a := range options.attachments {
		batch.Attachments = append(batch.Attachments, wireAttachment{
			Filename:      a.Filename,
			ContentBase64: base64.StdEncoding.EncodeToString(a.Content),
		})
	}

	if h := options.headers; h != nil && (len(h.To) > 0 || len(h.CC) > 0) {
		batch.Headers = &wireHeaders{To: h.To, CC: h.CC}
	}

	if u := options.oneClickUnsubscribe; u != nil {
		batch.OneClickUnsubscribe = &wireUnsubscribe{URLTemplate: u.URLTemplate}
	}

	return batch
}

// toResult fills each rejection's email from the request, since the server
// identifies rejected recipients by index.
func (c wireBatchCreated) toResult(to []Recipient, replayed bool) MessageResult {
	rejected := make([]RejectedRecipient, 0, len(c.RejectedRecipients))
	for _, r := range c.RejectedRecipients {
		email := r.Email
		if email == "" && r.Index >= 0 && r.Index < len(to) {
			email = to[r.Index].Email
		}
		rejected = append(rejected, RejectedRecipient{
			Index:  r.Index,
			Email:  email,
			Reason: RejectionReason(r.Code),
		})
	}

	return MessageResult{
		BatchID:            c.ID,
		ScheduledTime:      c.ScheduledAt,
		AcceptedCount:      c.AcceptedCount,
		RejectedCount:      c.RejectedCount,
		RejectedRecipients: rejected,
		Replayed:           replayed,
	}
}

func newIdempotencyKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("kannon: generate idempotency key: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
