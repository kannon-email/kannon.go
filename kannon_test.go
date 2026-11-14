package kannon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

type capturedRequest struct {
	method  string
	path    string
	headers http.Header
	body    map[string]any
}

type fakeServer struct {
	status   int
	headers  map[string]string
	response string
	got      capturedRequest
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	f.got = capturedRequest{method: r.Method, path: r.URL.EscapedPath(), headers: r.Header.Clone()}
	_ = json.Unmarshal(raw, &f.got.body)
	for k, v := range f.headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(f.status)
	_, _ = io.WriteString(w, f.response)
}

const createdResponse = `{
	"id": "batch-1",
	"domain": "example.com",
	"from": {"email": "sender@example.com"},
	"subject": "Subject",
	"status": "pending",
	"created_at": "2026-11-14T10:00:00Z",
	"scheduled_at": "2026-11-14T10:00:00Z",
	"completed_at": null,
	"expires_at": "2026-12-14T10:00:00Z",
	"accepted_count": 1,
	"rejected_count": 0,
	"delivery_counts": {"pending": 1},
	"rejected_recipients": []
}`

func newTestKannon(t *testing.T, status int, response string) (*Kannon, *fakeServer) {
	t.Helper()
	fake := &fakeServer{status: status, response: response}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	k := NewKannon(
		"example.com",
		"api-key",
		Sender{Email: "sender@example.com", Alias: "Test Sender"},
		WithBaseURL(srv.URL+"/v1/"),
		WithHTTPClient(srv.Client()),
	)
	return k, fake
}

func toJSON(t *testing.T, v any) string {
	t.Helper()
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func TestSendEmailRequest(t *testing.T) {
	k, fake := newTestKannon(t, http.StatusCreated, createdResponse)

	_, err := k.SendEmail(context.Background(), []Recipient{{Email: "user@example.com"}}, "Subject", "<p>Hello</p>")
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}

	if fake.got.method != http.MethodPost || fake.got.path != "/v1/domains/example.com/batches" {
		t.Errorf("request = %s %s", fake.got.method, fake.got.path)
	}
	if got := fake.got.headers.Get("Authorization"); got != "Bearer api-key" {
		t.Errorf("Authorization = %q", got)
	}
	if got := fake.got.headers.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if got := fake.got.headers.Get("Idempotency-Key"); !uuid.MatchString(got) {
		t.Errorf("Idempotency-Key = %q, want a random UUID", got)
	}

	want := `{"content":{"html":"<p>Hello</p>","type":"html"},"from":{"email":"sender@example.com","name":"Test Sender"},"recipients":[{"email":"user@example.com"}],"subject":"Subject"}`
	if got := toJSON(t, fake.got.body); got != want {
		t.Errorf("body = %s\nwant %s", got, want)
	}
}

func TestSendTemplateRequestCarriesOptions(t *testing.T) {
	k, fake := newTestKannon(t, http.StatusCreated, createdResponse)
	scheduled := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

	_, err := k.SendTemplate(
		context.Background(),
		[]Recipient{{
			Email:    "user@example.com",
			Fields:   Fields{"name": "Ada"},
			Tracking: &TrackingPolicy{Opens: TrackingModeOff},
		}},
		"Subject",
		"tpl-1",
		WithAttachments([]Attachment{{Filename: "a.txt", Content: []byte("hi")}}),
		WithGlobalFields(map[string]string{"company": "ACME"}),
		WithHeaders(Headers{To: []string{"team@example.com"}, CC: []string{"boss@example.com"}}),
		WithScheduledTime(scheduled),
		WithTracking(TrackingPolicy{Opens: TrackingModeAnonymous, Links: TrackingModeIdentified}),
		WithOneClickUnsubscribe("https://example.com/unsub?e={{ email }}"),
		WithReplyTo("support@example.com", "Support"),
		WithIdempotencyKey("my-key"),
	)
	if err != nil {
		t.Fatalf("SendTemplate: %v", err)
	}

	if got := fake.got.headers.Get("Idempotency-Key"); got != "my-key" {
		t.Errorf("Idempotency-Key = %q", got)
	}

	want := `{"attachments":[{"content_base64":"aGk=","filename":"a.txt"}],` +
		`"content":{"template_id":"tpl-1","type":"template"},` +
		`"from":{"email":"sender@example.com","name":"Test Sender"},` +
		`"global_fields":{"company":"ACME"},` +
		`"headers":{"cc":["boss@example.com"],"to":["team@example.com"]},` +
		`"one_click_unsubscribe":{"url_template":"https://example.com/unsub?e={{ email }}"},` +
		`"recipients":[{"email":"user@example.com","fields":{"name":"Ada"},"tracking":{"opens":"off"}}],` +
		`"reply_to":{"email":"support@example.com","name":"Support"},` +
		`"scheduled_at":"2030-01-02T03:04:05Z",` +
		`"subject":"Subject",` +
		`"tracking":{"links":"identified","opens":"anonymous"}}`
	if got := toJSON(t, fake.got.body); got != want {
		t.Errorf("body = %s\nwant %s", got, want)
	}
}

func TestDomainIsPathEscaped(t *testing.T) {
	k, fake := newTestKannon(t, http.StatusCreated, createdResponse)
	k.domain = "a/b"

	if _, err := k.SendEmail(context.Background(), []Recipient{{Email: "u@example.com"}}, "S", "<p>x</p>"); err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if fake.got.path != "/v1/domains/a%2Fb/batches" {
		t.Errorf("path = %q", fake.got.path)
	}
}

func TestEmptyHeadersAreOmitted(t *testing.T) {
	k, fake := newTestKannon(t, http.StatusCreated, createdResponse)

	if _, err := k.SendEmail(context.Background(), []Recipient{{Email: "u@example.com"}}, "S", "<p>x</p>", WithHeaders(Headers{})); err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if _, ok := fake.got.body["headers"]; ok {
		t.Errorf("headers should be omitted, got %v", fake.got.body["headers"])
	}
}

func TestResultMapping(t *testing.T) {
	k, _ := newTestKannon(t, http.StatusCreated, `{
		"id": "batch-1",
		"scheduled_at": "2030-01-02T03:04:05Z",
		"accepted_count": 1,
		"rejected_count": 2,
		"rejected_recipients": [
			{"index": 1, "code": "invalid_email", "pointer": "/recipients/1/email", "detail": "Recipient refused: invalid_email"},
			{"index": 2, "code": "tracking_above_ceiling", "pointer": "/recipients/2/tracking", "detail": "Recipient refused: tracking_above_ceiling"}
		]
	}`)

	result, err := k.SendEmail(context.Background(), []Recipient{
		{Email: "ok@example.com"},
		{Email: "not-an-email"},
		{Email: "opted-out@example.com"},
	}, "S", "<p>x</p>")
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}

	if result.BatchID != "batch-1" || result.AcceptedCount != 1 || result.RejectedCount != 2 || result.Replayed {
		t.Errorf("result = %+v", result)
	}
	if !result.ScheduledTime.Equal(time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("scheduledTime = %v", result.ScheduledTime)
	}
	want := []RejectedRecipient{
		{Index: 1, Email: "not-an-email", Reason: RejectionReasonInvalidEmail},
		{Index: 2, Email: "opted-out@example.com", Reason: RejectionReasonTrackingAboveCeiling},
	}
	if len(result.RejectedRecipients) != len(want) {
		t.Fatalf("rejectedRecipients = %+v", result.RejectedRecipients)
	}
	for i := range want {
		if result.RejectedRecipients[i] != want[i] {
			t.Errorf("rejectedRecipients[%d] = %+v, want %+v", i, result.RejectedRecipients[i], want[i])
		}
	}
}

func TestReplay(t *testing.T) {
	k, _ := newTestKannon(t, http.StatusOK, `{
		"id": "batch-1",
		"domain": "example.com",
		"created_at": "2026-11-14T10:00:00Z",
		"scheduled_at": "2026-11-14T10:00:00Z",
		"accepted_count": 3,
		"rejected_count": 1
	}`)

	result, err := k.SendEmail(context.Background(), []Recipient{{Email: "u@example.com"}}, "S", "<p>x</p>", WithIdempotencyKey("k"))
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if !result.Replayed || result.BatchID != "batch-1" || result.AcceptedCount != 3 || result.RejectedCount != 1 {
		t.Errorf("result = %+v", result)
	}
	if result.RejectedRecipients == nil || len(result.RejectedRecipients) != 0 {
		t.Errorf("rejectedRecipients = %#v, want an empty slice", result.RejectedRecipients)
	}
}

func TestNoRecipientsAcceptedIsAnAPIError(t *testing.T) {
	k, fake := newTestKannon(t, http.StatusUnprocessableEntity, `{
		"type": "urn:kannon:problem:no_recipients_accepted",
		"title": "Unprocessable Entity",
		"status": 422,
		"code": "no_recipients_accepted",
		"request_id": "req-1",
		"errors": [{"code": "invalid_email", "pointer": "/recipients/0/email", "detail": "Recipient refused: invalid_email"}]
	}`)
	fake.headers = map[string]string{"Content-Type": "application/problem+json"}

	_, err := k.SendEmail(context.Background(), []Recipient{{Email: "nope"}}, "S", "<p>x</p>")

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.StatusCode != 422 || apiErr.Code != CodeNoRecipientsAccepted || apiErr.RequestID != "req-1" {
		t.Errorf("apiErr = %+v", apiErr)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0] != (FieldError{Code: "invalid_email", Pointer: "/recipients/0/email", Detail: "Recipient refused: invalid_email"}) {
		t.Errorf("errors = %+v", apiErr.Errors)
	}
}

func TestAPIErrorRetryAfterAndFallback(t *testing.T) {
	k, fake := newTestKannon(t, http.StatusServiceUnavailable, "upstream down")
	fake.headers = map[string]string{"Retry-After": "7", "X-Request-ID": "req-2"}

	_, err := k.SendEmail(context.Background(), []Recipient{{Email: "u@example.com"}}, "S", "<p>x</p>")

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.StatusCode != 503 || apiErr.Code != "" || apiErr.RetryAfter != 7*time.Second || apiErr.RequestID != "req-2" {
		t.Errorf("apiErr = %+v", apiErr)
	}
	if apiErr.Error() != "kannon: 503 Service Unavailable" {
		t.Errorf("Error() = %q", apiErr.Error())
	}
}

func TestDefaultBaseURL(t *testing.T) {
	k := NewKannon("example.com", "key", Sender{})
	if k.baseURL != DefaultBaseURL || k.httpClient != http.DefaultClient {
		t.Errorf("baseURL = %q, httpClient = %v", k.baseURL, k.httpClient)
	}
}
