package kannon

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/kannon-email/kannon.go/internal/proto/kannon/mailer/apiv1"
	trackingtypes "github.com/kannon-email/kannon.go/internal/proto/kannon/tracking/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fakeMailerClient captures the outgoing request and returns a canned
// response, so tests can assert on the wire mapping without a server.
type fakeMailerClient struct {
	htmlReq     *connect.Request[apiv1.SendHTMLReq]
	templateReq *connect.Request[apiv1.SendTemplateReq]
	res         *apiv1.SendRes
}

func (f *fakeMailerClient) SendHTML(_ context.Context, req *connect.Request[apiv1.SendHTMLReq]) (*connect.Response[apiv1.SendRes], error) {
	f.htmlReq = req
	return connect.NewResponse(f.res), nil
}

func (f *fakeMailerClient) SendTemplate(_ context.Context, req *connect.Request[apiv1.SendTemplateReq]) (*connect.Response[apiv1.SendRes], error) {
	f.templateReq = req
	return connect.NewResponse(f.res), nil
}

func emptyResponse() *apiv1.SendRes {
	return &apiv1.SendRes{
		MessageId:          "message-id",
		TemplateId:         "template-id",
		ScheduledTime:      nil,
		AcceptedCount:      0,
		RejectedCount:      0,
		RejectedRecipients: nil,
	}
}

func newTestKannon(res *apiv1.SendRes) (*Kannon, *fakeMailerClient) {
	cli := &fakeMailerClient{res: res}
	k := &Kannon{
		domain: "example.com",
		key:    "api-key",
		sender: Sender{Email: "sender@example.com", Alias: "Test Sender"},
		cli:    cli,
	}
	return k, cli
}

func TestSendEmailDefaults(t *testing.T) {
	k, cli := newTestKannon(emptyResponse())

	_, err := k.SendEmail(context.Background(), []Recipient{{Email: "user@example.com"}}, "Subject", "<p>Hello</p>")
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}

	msg := cli.htmlReq.Msg
	if msg.Tracking != nil {
		t.Errorf("tracking should be omitted, got %v", msg.Tracking)
	}
	if msg.Headers != nil {
		t.Errorf("headers should be omitted, got %v", msg.Headers)
	}
	if msg.OneClickUnsubscribe != nil {
		t.Errorf("oneClickUnsubscribe should be omitted, got %v", msg.OneClickUnsubscribe)
	}
	if msg.ScheduledTime != nil {
		t.Errorf("scheduledTime should be omitted so the server sends now, got %v", msg.ScheduledTime)
	}
	if msg.Attachments == nil || len(msg.Attachments) != 0 {
		t.Errorf("attachments should be an empty slice, got %v", msg.Attachments)
	}
	if msg.GlobalFields == nil || len(msg.GlobalFields) != 0 {
		t.Errorf("globalFields should be an empty map, got %v", msg.GlobalFields)
	}
	if msg.Html != "<p>Hello</p>" {
		t.Errorf("html = %q, want %q", msg.Html, "<p>Hello</p>")
	}
	if msg.Subject != "Subject" {
		t.Errorf("subject = %q, want %q", msg.Subject, "Subject")
	}
	if msg.Sender.GetEmail() != "sender@example.com" || msg.Sender.GetAlias() != "Test Sender" {
		t.Errorf("sender = %v, want sender@example.com / Test Sender", msg.Sender)
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("example.com:api-key"))
	if got := cli.htmlReq.Header().Get("Authorization"); got != wantAuth {
		t.Errorf("authorization = %q, want %q", got, wantAuth)
	}
}

func TestSendEmailWithTracking(t *testing.T) {
	tests := []struct {
		name      string
		policy    TrackingPolicy
		wantOpens trackingtypes.TrackingMode
		wantLinks trackingtypes.TrackingMode
	}{
		{
			name:      "only opens set leaves links unspecified",
			policy:    TrackingPolicy{Opens: TrackingModeIdentified},
			wantOpens: trackingtypes.TrackingMode_TRACKING_MODE_IDENTIFIED,
			wantLinks: trackingtypes.TrackingMode_TRACKING_MODE_UNSPECIFIED,
		},
		{
			name:      "both axes set",
			policy:    TrackingPolicy{Opens: TrackingModeAnonymous, Links: TrackingModeOff},
			wantOpens: trackingtypes.TrackingMode_TRACKING_MODE_ANONYMOUS,
			wantLinks: trackingtypes.TrackingMode_TRACKING_MODE_OFF,
		},
		{
			name:      "empty policy is still sent with both axes unspecified",
			policy:    TrackingPolicy{},
			wantOpens: trackingtypes.TrackingMode_TRACKING_MODE_UNSPECIFIED,
			wantLinks: trackingtypes.TrackingMode_TRACKING_MODE_UNSPECIFIED,
		},
		{
			name:      "full modes map",
			policy:    TrackingPolicy{Opens: TrackingModeFull, Links: TrackingModePseudonymous},
			wantOpens: trackingtypes.TrackingMode_TRACKING_MODE_FULL,
			wantLinks: trackingtypes.TrackingMode_TRACKING_MODE_PSEUDONYMOUS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, cli := newTestKannon(emptyResponse())

			_, err := k.SendEmail(context.Background(), []Recipient{{Email: "user@example.com"}}, "Subject", "<p>Hello</p>", WithTracking(tt.policy))
			if err != nil {
				t.Fatalf("SendEmail: %v", err)
			}

			tracking := cli.htmlReq.Msg.Tracking
			if tracking == nil {
				t.Fatal("tracking should be present when the option is set")
			}
			if tracking.Opens != tt.wantOpens {
				t.Errorf("opens = %v, want %v", tracking.Opens, tt.wantOpens)
			}
			if tracking.Links != tt.wantLinks {
				t.Errorf("links = %v, want %v", tracking.Links, tt.wantLinks)
			}
		})
	}
}

func TestRecipientTracking(t *testing.T) {
	tests := []struct {
		name      string
		recipient Recipient
		want      *trackingtypes.TrackingPolicy
	}{
		{
			name:      "nil tracking states nothing",
			recipient: Recipient{Email: "user@example.com"},
			want:      nil,
		},
		{
			name: "tracking policy is mapped",
			recipient: Recipient{
				Email:    "user@example.com",
				Tracking: &TrackingPolicy{Opens: TrackingModeOff, Links: TrackingModeAnonymous},
			},
			want: &trackingtypes.TrackingPolicy{
				Opens: trackingtypes.TrackingMode_TRACKING_MODE_OFF,
				Links: trackingtypes.TrackingMode_TRACKING_MODE_ANONYMOUS,
			},
		},
		{
			name: "omitted axis is unspecified",
			recipient: Recipient{
				Email:    "user@example.com",
				Tracking: &TrackingPolicy{Opens: TrackingModeOff},
			},
			want: &trackingtypes.TrackingPolicy{
				Opens: trackingtypes.TrackingMode_TRACKING_MODE_OFF,
				Links: trackingtypes.TrackingMode_TRACKING_MODE_UNSPECIFIED,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, cli := newTestKannon(emptyResponse())

			_, err := k.SendEmail(context.Background(), []Recipient{tt.recipient}, "Subject", "<p>Hello</p>")
			if err != nil {
				t.Fatalf("SendEmail: %v", err)
			}

			got := cli.htmlReq.Msg.Recipients[0].Tracking
			if tt.want == nil {
				if got != nil {
					t.Fatalf("recipient tracking should be omitted, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("recipient tracking should be present")
			}
			if got.Opens != tt.want.Opens || got.Links != tt.want.Links {
				t.Errorf("recipient tracking = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSendEmailWithHeaders(t *testing.T) {
	k, cli := newTestKannon(emptyResponse())

	_, err := k.SendEmail(
		context.Background(),
		[]Recipient{{Email: "user@example.com"}},
		"Subject", "<p>Hello</p>",
		WithHeaders(Headers{To: []string{"team@example.com"}}),
	)
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}

	headers := cli.htmlReq.Msg.Headers
	if headers == nil {
		t.Fatal("headers should be present when the option is set")
	}
	if len(headers.To) != 1 || headers.To[0] != "team@example.com" {
		t.Errorf("headers.to = %v, want [team@example.com]", headers.To)
	}
	if headers.Cc == nil || len(headers.Cc) != 0 {
		t.Errorf("headers.cc should be an empty slice, got %v", headers.Cc)
	}
}

func TestSendEmailWithScheduledTime(t *testing.T) {
	k, cli := newTestKannon(emptyResponse())

	scheduled := time.Date(2024, 12, 25, 10, 0, 0, 0, time.UTC)
	_, err := k.SendEmail(
		context.Background(),
		[]Recipient{{Email: "user@example.com"}},
		"Subject", "<p>Hello</p>",
		WithScheduledTime(scheduled),
	)
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}

	got := cli.htmlReq.Msg.ScheduledTime
	if got == nil {
		t.Fatal("scheduledTime should be present when the option is set")
	}
	if !got.AsTime().Equal(scheduled) {
		t.Errorf("scheduledTime = %v, want %v", got.AsTime(), scheduled)
	}
}

func TestSendEmailWithOneClickUnsubscribe(t *testing.T) {
	k, cli := newTestKannon(emptyResponse())

	_, err := k.SendEmail(
		context.Background(),
		[]Recipient{{Email: "user@example.com"}},
		"Subject", "<p>Hello</p>",
		WithOneClickUnsubscribe("https://example.com/unsub?email={{ email }}"),
	)
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}

	unsub := cli.htmlReq.Msg.OneClickUnsubscribe
	if unsub == nil {
		t.Fatal("oneClickUnsubscribe should be present when the option is set")
	}
	if unsub.UrlTemplate != "https://example.com/unsub?email={{ email }}" {
		t.Errorf("urlTemplate = %q", unsub.UrlTemplate)
	}
}

func TestSendEmailWithAttachmentsAndGlobalFields(t *testing.T) {
	k, cli := newTestKannon(emptyResponse())

	_, err := k.SendEmail(
		context.Background(),
		[]Recipient{{Email: "user@example.com"}},
		"Subject", "<p>Hello</p>",
		WithAttachments([]Attachment{{Filename: "report.pdf", Content: []byte("content")}}),
		WithGlobalFields(map[string]string{"company": "Kannon"}),
	)
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}

	msg := cli.htmlReq.Msg
	if len(msg.Attachments) != 1 || msg.Attachments[0].Filename != "report.pdf" || string(msg.Attachments[0].Content) != "content" {
		t.Errorf("attachments = %v", msg.Attachments)
	}
	if len(msg.GlobalFields) != 1 || msg.GlobalFields["company"] != "Kannon" {
		t.Errorf("globalFields = %v", msg.GlobalFields)
	}
}

func TestSendTemplateCarriesOptions(t *testing.T) {
	k, cli := newTestKannon(emptyResponse())

	_, err := k.SendTemplate(
		context.Background(),
		[]Recipient{{Email: "user@example.com", Tracking: &TrackingPolicy{Opens: TrackingModeOff}}},
		"Subject", "template-id",
		WithTracking(TrackingPolicy{Opens: TrackingModeIdentified}),
		WithOneClickUnsubscribe("https://example.com/unsub"),
	)
	if err != nil {
		t.Fatalf("SendTemplate: %v", err)
	}

	msg := cli.templateReq.Msg
	if msg.TemplateId != "template-id" {
		t.Errorf("templateId = %q, want %q", msg.TemplateId, "template-id")
	}
	if msg.Tracking == nil || msg.Tracking.Opens != trackingtypes.TrackingMode_TRACKING_MODE_IDENTIFIED {
		t.Errorf("tracking = %v, want opens identified", msg.Tracking)
	}
	if msg.OneClickUnsubscribe == nil || msg.OneClickUnsubscribe.UrlTemplate != "https://example.com/unsub" {
		t.Errorf("oneClickUnsubscribe = %v", msg.OneClickUnsubscribe)
	}
	if rt := msg.Recipients[0].Tracking; rt == nil || rt.Opens != trackingtypes.TrackingMode_TRACKING_MODE_OFF {
		t.Errorf("recipient tracking = %v, want opens off", rt)
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("example.com:api-key"))
	if got := cli.templateReq.Header().Get("Authorization"); got != wantAuth {
		t.Errorf("authorization = %q, want %q", got, wantAuth)
	}
}

func TestResultMapping(t *testing.T) {
	scheduled := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	res := &apiv1.SendRes{
		MessageId:     "message-id",
		TemplateId:    "template-id",
		ScheduledTime: timestamppb.New(scheduled),
		AcceptedCount: 1,
		RejectedCount: 2,
		RejectedRecipients: []*apiv1.RejectedRecipient{
			{Email: "not-an-email", Reason: "invalid_email"},
			{Email: "opted-out@example.com", Reason: "tracking_above_ceiling"},
		},
	}
	k, _ := newTestKannon(res)

	result, err := k.SendEmail(context.Background(), []Recipient{{Email: "user@example.com"}}, "Subject", "<p>Hello</p>")
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}

	if result.MessageID != "message-id" {
		t.Errorf("messageID = %q", result.MessageID)
	}
	if result.TemplateID != "template-id" {
		t.Errorf("templateID = %q", result.TemplateID)
	}
	if !result.ScheduledTime.Equal(scheduled) {
		t.Errorf("scheduledTime = %v, want %v", result.ScheduledTime, scheduled)
	}
	if result.AcceptedCount != 1 {
		t.Errorf("acceptedCount = %d, want 1", result.AcceptedCount)
	}
	if result.RejectedCount != 2 {
		t.Errorf("rejectedCount = %d, want 2", result.RejectedCount)
	}
	want := []RejectedRecipient{
		{Email: "not-an-email", Reason: RejectionReasonInvalidEmail},
		{Email: "opted-out@example.com", Reason: RejectionReasonTrackingAboveCeiling},
	}
	if len(result.RejectedRecipients) != len(want) {
		t.Fatalf("rejectedRecipients = %v, want %v", result.RejectedRecipients, want)
	}
	for i, r := range want {
		if result.RejectedRecipients[i] != r {
			t.Errorf("rejectedRecipients[%d] = %v, want %v", i, result.RejectedRecipients[i], r)
		}
	}
}

func TestResultMappingNilScheduledTime(t *testing.T) {
	k, _ := newTestKannon(emptyResponse())

	before := time.Now()
	result, err := k.SendEmail(context.Background(), []Recipient{{Email: "user@example.com"}}, "Subject", "<p>Hello</p>")
	after := time.Now()
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}

	if result.ScheduledTime.Before(before) || result.ScheduledTime.After(after) {
		t.Errorf("scheduledTime = %v, want between %v and %v", result.ScheduledTime, before, after)
	}
}

func TestRecipientFieldsDefaultToEmptyMap(t *testing.T) {
	k, cli := newTestKannon(emptyResponse())

	_, err := k.SendEmail(
		context.Background(),
		[]Recipient{
			{Email: "user@example.com"},
			{Email: "other@example.com", Fields: Fields{"name": "John"}},
		},
		"Subject", "<p>Hello</p>",
	)
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}

	recipients := cli.htmlReq.Msg.Recipients
	if recipients[0].Fields == nil || len(recipients[0].Fields) != 0 {
		t.Errorf("fields should default to an empty map, got %v", recipients[0].Fields)
	}
	if recipients[1].Fields["name"] != "John" {
		t.Errorf("fields = %v, want name=John", recipients[1].Fields)
	}
}
