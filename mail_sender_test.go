package kannon

import (
	"context"
	"testing"
	"time"
)

// fakeHTMLSender captures the arguments MailSender passes to the client and
// returns a canned result.
type fakeHTMLSender struct {
	to      []Recipient
	subject string
	body    string
	opts    sendOptions
	result  MessageResult
}

func (f *fakeHTMLSender) SendEmail(_ context.Context, to []Recipient, subject, body string, opts ...SendOption) (MessageResult, error) {
	f.to = to
	f.subject = subject
	f.body = body
	f.opts = applySendOptions(opts)
	return f.result, nil
}

func defaultResult() MessageResult {
	return MessageResult{
		MessageID:          "test-id",
		TemplateID:         "",
		ScheduledTime:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		AcceptedCount:      1,
		RejectedCount:      0,
		RejectedRecipients: []RejectedRecipient{},
	}
}

func newTestMailSender() (*MailSender, *fakeHTMLSender) {
	client := &fakeHTMLSender{result: defaultResult()}
	return NewMailSender(client), client
}

func emails(recipients []Recipient) []string {
	res := make([]string, 0, len(recipients))
	for _, r := range recipients {
		res = append(res, r.Email)
	}
	return res
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMailSenderRecipientsAndHeaders(t *testing.T) {
	tests := []struct {
		name           string
		params         SendParams
		wantRecipients []string
		wantHeaderTo   []string
		wantHeaderCC   []string
	}{
		{
			name:           "single to recipient",
			params:         SendParams{To: []string{"user@example.com"}},
			wantRecipients: []string{"user@example.com"},
			wantHeaderTo:   []string{"user@example.com"},
			wantHeaderCC:   nil,
		},
		{
			name:           "multiple to recipients",
			params:         SendParams{To: []string{"user1@example.com", "user2@example.com"}},
			wantRecipients: []string{"user1@example.com", "user2@example.com"},
			wantHeaderTo:   []string{"user1@example.com", "user2@example.com"},
			wantHeaderCC:   nil,
		},
		{
			name: "cc recipients appear in recipients and headers",
			params: SendParams{
				To: []string{"user@example.com"},
				CC: []string{"cc@example.com"},
			},
			wantRecipients: []string{"user@example.com", "cc@example.com"},
			wantHeaderTo:   []string{"user@example.com"},
			wantHeaderCC:   []string{"cc@example.com"},
		},
		{
			name: "multiple cc recipients",
			params: SendParams{
				To: []string{"user@example.com"},
				CC: []string{"cc1@example.com", "cc2@example.com"},
			},
			wantRecipients: []string{"user@example.com", "cc1@example.com", "cc2@example.com"},
			wantHeaderTo:   []string{"user@example.com"},
			wantHeaderCC:   []string{"cc1@example.com", "cc2@example.com"},
		},
		{
			name: "bcc recipients receive the mail but never appear in headers",
			params: SendParams{
				To:  []string{"user@example.com"},
				BCC: []string{"bcc@example.com"},
			},
			wantRecipients: []string{"user@example.com", "bcc@example.com"},
			wantHeaderTo:   []string{"user@example.com"},
			wantHeaderCC:   nil,
		},
		{
			name: "all recipient types together, in to-cc-bcc order",
			params: SendParams{
				To:  []string{"to1@example.com", "to2@example.com"},
				CC:  []string{"cc@example.com"},
				BCC: []string{"bcc1@example.com", "bcc2@example.com"},
			},
			wantRecipients: []string{
				"to1@example.com",
				"to2@example.com",
				"cc@example.com",
				"bcc1@example.com",
				"bcc2@example.com",
			},
			wantHeaderTo: []string{"to1@example.com", "to2@example.com"},
			wantHeaderCC: []string{"cc@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender, client := newTestMailSender()

			tt.params.Subject = "Test"
			tt.params.Content = "<p>Hello</p>"
			if _, err := sender.Send(context.Background(), tt.params); err != nil {
				t.Fatalf("Send: %v", err)
			}

			if got := emails(client.to); !equalStrings(got, tt.wantRecipients) {
				t.Errorf("recipients = %v, want %v", got, tt.wantRecipients)
			}
			if client.opts.headers == nil {
				t.Fatal("headers should always be set")
			}
			if !equalStrings(client.opts.headers.To, tt.wantHeaderTo) {
				t.Errorf("headers.To = %v, want %v", client.opts.headers.To, tt.wantHeaderTo)
			}
			if !equalStrings(client.opts.headers.CC, tt.wantHeaderCC) {
				t.Errorf("headers.CC = %v, want %v", client.opts.headers.CC, tt.wantHeaderCC)
			}
			if client.subject != "Test" {
				t.Errorf("subject = %q, want %q", client.subject, "Test")
			}
			if client.body != "<p>Hello</p>" {
				t.Errorf("body = %q, want %q", client.body, "<p>Hello</p>")
			}
		})
	}
}

func TestMailSenderPassesAttachmentsThrough(t *testing.T) {
	sender, client := newTestMailSender()

	attachments := []Attachment{{Filename: "test.pdf", Content: []byte("test content")}}
	_, err := sender.Send(context.Background(), SendParams{
		To:          []string{"user@example.com"},
		Subject:     "Test",
		Content:     "<p>Hello</p>",
		Attachments: attachments,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(client.opts.attachments) != 1 ||
		client.opts.attachments[0].Filename != "test.pdf" ||
		string(client.opts.attachments[0].Content) != "test content" {
		t.Errorf("attachments = %v, want %v", client.opts.attachments, attachments)
	}
}

func TestMailSenderPassesScheduledTimeThrough(t *testing.T) {
	sender, client := newTestMailSender()

	scheduled := time.Date(2024, 12, 25, 10, 0, 0, 0, time.UTC)
	_, err := sender.Send(context.Background(), SendParams{
		To:            []string{"user@example.com"},
		Subject:       "Test",
		Content:       "<p>Hello</p>",
		ScheduledTime: &scheduled,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if client.opts.scheduledTime == nil || !client.opts.scheduledTime.Equal(scheduled) {
		t.Errorf("scheduledTime = %v, want %v", client.opts.scheduledTime, scheduled)
	}
}

func TestMailSenderOmitsScheduledTimeWhenUnset(t *testing.T) {
	sender, client := newTestMailSender()

	_, err := sender.Send(context.Background(), SendParams{
		To:      []string{"user@example.com"},
		Subject: "Test",
		Content: "<p>Hello</p>",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if client.opts.scheduledTime != nil {
		t.Errorf("scheduledTime should be unset, got %v", client.opts.scheduledTime)
	}
}

func TestMailSenderAlwaysDisablesTracking(t *testing.T) {
	sender, client := newTestMailSender()

	_, err := sender.Send(context.Background(), SendParams{
		To:      []string{"user@example.com"},
		Subject: "Test",
		Content: "<p>Hello</p>",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if client.opts.tracking == nil {
		t.Fatal("tracking should always be stated")
	}
	if client.opts.tracking.Opens != TrackingModeOff || client.opts.tracking.Links != TrackingModeOff {
		t.Errorf("tracking = %+v, want opens and links off", client.opts.tracking)
	}
}

func TestMailSenderNeverEmitsUnsubscribeOrGlobalFields(t *testing.T) {
	sender, client := newTestMailSender()

	_, err := sender.Send(context.Background(), SendParams{
		To:      []string{"user@example.com"},
		Subject: "Test",
		Content: "<p>Hello</p>",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if client.opts.oneClickUnsubscribe != nil {
		t.Errorf("oneClickUnsubscribe should never be set, got %v", client.opts.oneClickUnsubscribe)
	}
	if client.opts.globalFields != nil {
		t.Errorf("globalFields should never be set, got %v", client.opts.globalFields)
	}
}

func TestMailSenderResultMapping(t *testing.T) {
	sender, _ := newTestMailSender()

	result, err := sender.Send(context.Background(), SendParams{
		To:      []string{"user@example.com"},
		Subject: "Test",
		Content: "<p>Hello</p>",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	want := SendResult{
		MessageID:          "test-id",
		AcceptedCount:      1,
		RejectedCount:      0,
		RejectedRecipients: []RejectedRecipient{},
		ScheduledTime:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if result.MessageID != want.MessageID ||
		result.AcceptedCount != want.AcceptedCount ||
		result.RejectedCount != want.RejectedCount ||
		len(result.RejectedRecipients) != 0 ||
		!result.ScheduledTime.Equal(want.ScheduledTime) {
		t.Errorf("result = %+v, want %+v", result, want)
	}
}

func TestMailSenderReportsRejectedRecipients(t *testing.T) {
	rejected := []RejectedRecipient{
		{Email: "not-an-email", Reason: RejectionReasonInvalidEmail},
		{Email: "opted-out@example.com", Reason: RejectionReasonTrackingAboveCeiling},
	}
	client := &fakeHTMLSender{result: MessageResult{
		MessageID:          "test-id",
		ScheduledTime:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		AcceptedCount:      1,
		RejectedCount:      2,
		RejectedRecipients: rejected,
	}}
	sender := NewMailSender(client)

	result, err := sender.Send(context.Background(), SendParams{
		To:      []string{"user@example.com", "not-an-email", "opted-out@example.com"},
		Subject: "Test",
		Content: "<p>Hello</p>",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if result.AcceptedCount != 1 {
		t.Errorf("acceptedCount = %d, want 1", result.AcceptedCount)
	}
	if result.RejectedCount != 2 {
		t.Errorf("rejectedCount = %d, want 2", result.RejectedCount)
	}
	if len(result.RejectedRecipients) != 2 ||
		result.RejectedRecipients[0] != rejected[0] ||
		result.RejectedRecipients[1] != rejected[1] {
		t.Errorf("rejectedRecipients = %v, want %v", result.RejectedRecipients, rejected)
	}
}
