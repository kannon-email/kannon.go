package kannon

import "time"

// Attachment is a file attached to every email of a batch.
type Attachment struct {
	// Filename is the name the file will carry in the email.
	Filename string
	// Content is the raw file content.
	Content []byte
}

// Headers customizes the To and Cc headers of the sent emails.
//
// Every address listed here is display only: it does not receive the email
// unless it is also part of the recipient list.
type Headers struct {
	// To is the address list shown in the To header.
	To []string
	// CC is the address list shown in the Cc header.
	CC []string
}

// OneClickUnsubscribe is your own one-click unsubscribe endpoint, carried in
// the List-Unsubscribe and List-Unsubscribe-Post headers (RFC 8058).
//
// Kannon personalises, emits and DKIM-signs it, and does nothing else with it:
// it never calls the endpoint, keeps no suppression list, and records no
// engagement when a recipient uses it.
type OneClickUnsubscribe struct {
	// URLTemplate is the https URL of the endpoint, as a template.
	//
	// The endpoint must accept POST with body `List-Unsubscribe=One-Click` and
	// unsubscribe the recipient with no further confirmation.
	//
	// `{{ field }}` placeholders are substituted per delivery from the
	// recipient's fields, plus `email`, which Kannon supplies as the
	// recipient's address unless you pass a field of that name yourself.
	// Substituted values are percent-encoded, so pass raw values rather than
	// pre-encoded ones.
	//
	// A template that is unparseable or not https fails the whole call. A
	// recipient whose fields leave a placeholder unresolved is rejected on its
	// own, with reason RejectionReasonUnsubscribeURLUnresolved.
	URLTemplate string
}

// SendOption customizes a single send performed by SendEmail or SendTemplate.
type SendOption func(*sendOptions)

type sendOptions struct {
	attachments         []Attachment
	globalFields        map[string]string
	headers             *Headers
	oneClickUnsubscribe *OneClickUnsubscribe
	scheduledTime       *time.Time
	tracking            *TrackingPolicy
}

func applySendOptions(opts []SendOption) sendOptions {
	var options sendOptions
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// WithAttachments attaches the given files to every email of the batch.
func WithAttachments(attachments []Attachment) SendOption {
	return func(o *sendOptions) {
		o.attachments = attachments
	}
}

// WithGlobalFields sets template fields shared by every recipient of the
// batch. A recipient's own fields take precedence on conflicts.
func WithGlobalFields(fields map[string]string) SendOption {
	return func(o *sendOptions) {
		o.globalFields = fields
	}
}

// WithHeaders customizes the To and Cc headers of the sent emails.
func WithHeaders(headers Headers) SendOption {
	return func(o *sendOptions) {
		o.headers = &headers
	}
}

// WithScheduledTime schedules the batch for future delivery. When omitted,
// the server sends immediately.
func WithScheduledTime(t time.Time) SendOption {
	return func(o *sendOptions) {
		o.scheduledTime = &t
	}
}

// WithTracking states the tracking policy for the batch.
//
// Omitted states nothing and resolves to the domain ceiling configured on the
// server. A mode above that ceiling fails the call.
func WithTracking(policy TrackingPolicy) SendOption {
	return func(o *sendOptions) {
		o.tracking = &policy
	}
}

// WithOneClickUnsubscribe emits your own one-click unsubscribe endpoint
// (RFC 8058) on every delivery of the batch. Kannon never adds one of its
// own. See OneClickUnsubscribe for the template rules.
func WithOneClickUnsubscribe(urlTemplate string) SendOption {
	return func(o *sendOptions) {
		o.oneClickUnsubscribe = &OneClickUnsubscribe{URLTemplate: urlTemplate}
	}
}
