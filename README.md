## kannon.go is the official GoLang client library for Kannon Email Sender

Instantiate kannon cli

```go
sender := kannon.Sender{
  Email: "sender@kannon.dev",
  Alias: "Kannon",
}

k := kannon.NewKannon(
  "<YOUR DOMAIN>",
  "<API KEY>",
  sender,
)
```

The client talks to the Kannon REST API v1 with only the Go standard library. It
targets the hosted API at `https://api.kannon.email/v1` by default; point it at
your own installation, or bring your own `*http.Client`, with client options:

```go
k := kannon.NewKannon(domain, key, sender,
  kannon.WithBaseURL("https://kannon.example.com/v1"),
  kannon.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
)
```
### Basic Usage

```go
html := `...`;

recipents := []kannon.Recipient{
  {
    Email: "test@email.com",
    Fields: kannon.Fields{
      "name": "Test",
    },
  },
}

res, err := k.SendEmail(
  context.Background(),
  recipents,
  "This is an email from kannon.go",
  html,
);
```

### Sending Templates

```go
templateID := `...`;

recipents := []kannon.Recipient{
  {
    Email: "test@email.com",
    Fields: kannon.Fields{
      "name": "Test",
    },
  },
}

res, err := k.SendTemplate(
  context.Background(),
  recipents,
  "This is an email from kannon.go",
  templateID,
);
```

### Send Options

Both `SendEmail` and `SendTemplate` accept functional options:

```go
res, err := k.SendEmail(
  context.Background(),
  recipents,
  "Monthly Report",
  html,
  kannon.WithAttachments([]kannon.Attachment{
    {Filename: "report.pdf", Content: pdfBytes},
  }),
  kannon.WithGlobalFields(map[string]string{
    "company": "ACME Corp",
  }),
  kannon.WithHeaders(kannon.Headers{
    To: []string{"team@example.com"},
    CC: []string{"manager@example.com"},
  }),
  kannon.WithScheduledTime(time.Now().Add(24*time.Hour)),
  kannon.WithReplyTo("support@example.com", "Support"),
  kannon.WithIdempotencyKey(orderID),
)
```

- `WithAttachments` attaches files to every email of the batch.
- `WithGlobalFields` sets template fields shared by every recipient; a recipient's own fields win on conflicts.
- `WithHeaders` customizes the To and Cc headers. Addresses listed here are display only: they do not receive the email unless they are also recipients.
- `WithScheduledTime` schedules the batch for future delivery. When omitted, the server sends immediately.
- `WithReplyTo` emits a `Reply-To` header, so replies go somewhere other than the sender.
- `WithIdempotencyKey` makes the send safe to retry: the server creates at most one batch per key for at least 24 hours, and answers a repeat with the original receipt (`res.Replayed` is true, `RejectedRecipients` is empty). Without it every call uses a fresh key, so retrying a call that timed out may send twice.

Recipient `Fields` personalise the subject and content. `email` is reserved for the recipient's address: setting it rejects that recipient with reason `reserved_field`.

### Tracking Policy

How much of an email's engagement may be observed is a policy stated at three levels — domain, batch and recipient. The effective policy is the **most restrictive** of the three: a level closer to the recipient may only narrow what the level above allows, never widen it.

The modes, ordered by increasing collection:

| Mode | Meaning |
|------|---------|
| `TrackingModeOff` | The channel is not observed at all |
| `TrackingModeAnonymous` | Counted in aggregate only; nothing retained that isolates one recipient |
| `TrackingModePseudonymous` | Reserved by the server: selecting it rejects the recipient |
| `TrackingModeIdentified` | Attributed to the recipient |
| `TrackingModeFull` | Attributed, plus the IP address and user agent of the request |

Leaving an axis at its zero value, or omitting the policy altogether, states nothing and imposes no restriction of its own: the domain ceiling configured on the server decides.

#### Per Batch

```go
res, err := k.SendEmail(ctx, recipents, "Password Reset", html,
  kannon.WithTracking(kannon.TrackingPolicy{
    Opens: kannon.TrackingModeOff,
    Links: kannon.TrackingModeOff,
  }),
)
```

A batch asking for more than its domain allows **fails the whole call**.

#### Per Recipient

State here the consent you hold in your own CRM:

```go
recipents := []kannon.Recipient{
  {
    Email: "consented@example.com",
    Tracking: &kannon.TrackingPolicy{
      Opens: kannon.TrackingModeIdentified,
      Links: kannon.TrackingModeIdentified,
    },
  },
  {
    Email: "opted-out@example.com",
    Tracking: &kannon.TrackingPolicy{
      Opens: kannon.TrackingModeOff,
      Links: kannon.TrackingModeOff,
    },
  },
}
```

A recipient asking for more than its domain allows is rejected on its own, with reason `tracking_above_ceiling`, while the rest of the batch proceeds.

### One-Click Unsubscribe (RFC 8058)

Kannon can carry a `List-Unsubscribe` / `List-Unsubscribe-Post` pair pointing at **your own** endpoint. It personalises the URL, emits it and DKIM-signs it, and does nothing else with it: it never calls the endpoint, keeps no suppression list, and records no engagement when a recipient uses it.

```go
res, err := k.SendEmail(ctx, recipents, "Newsletter", html,
  kannon.WithOneClickUnsubscribe(
    "https://your-domain.com/unsub?email={{ email }}&token={{ token }}",
  ),
)
```

Four things the API holds you to:

1. **The URL must be `https`.** A template that is malformed, relative or non-https fails the whole call.
2. **Your endpoint must accept `POST`** with body `List-Unsubscribe=One-Click` and unsubscribe with no further confirmation.
3. **Pass raw field values, not pre-encoded ones.** Kannon percent-encodes every value it substitutes.
4. **Every placeholder must resolve for every recipient.** One whose fields leave a placeholder unresolved is rejected with reason `unsubscribe_url_unresolved`. `{{ email }}` always resolves; a custom `{{ token }}` only does if you pass it for that recipient.

There is no domain-wide default and no per-recipient override: an unsubscribe header on a password reset offers the recipient something you cannot honour.

### Intake Results

A recipient refused at intake does not fail the send: it is reported back on `MessageResult`, with its index in your list, so you can reconcile what was actually queued.

```go
res, err := k.SendEmail(ctx, recipents, "Newsletter", html)
if err != nil {
  log.Fatal(err)
}

fmt.Printf("%d queued, %d rejected\n", res.AcceptedCount, res.RejectedCount)

for _, r := range res.RejectedRecipients {
  log.Printf("%s was rejected: %s", r.Email, r.Reason)
}
```

| Reason | Meaning |
|--------|---------|
| `invalid_email` | The address is empty or otherwise unusable |
| `duplicate_recipient` | The address already appears earlier in the batch |
| `tracking_above_ceiling` | The recipient's policy asks for more than its domain allows |
| `unsupported_tracking_mode` | The recipient stated a mode the server will not act on |
| `unsubscribe_url_unresolved` | A placeholder in the unsubscribe URL template was left unresolved |
| `unresolved_placeholder` | A placeholder in the subject or content was left unresolved |
| `reserved_field` | The recipient's fields override a reserved field, such as `email` |
| `message_too_large` | The personalised message exceeds the size the server accepts |

Treat an unrecognised value as a refusal of unknown cause: the set grows as new causes are added.

### Errors

A refused request returns an `*kannon.APIError` carrying the HTTP status, the stable problem `Code`, the server's `RequestID`, the offending `Errors` and any `RetryAfter` hint. When **every** recipient is refused no batch is created and the call fails with code `no_recipients_accepted`; each entry of `Errors` then names one recipient by pointer (`/recipients/<index>/...`) with its rejection reason as code.

```go
var apiErr *kannon.APIError
if errors.As(err, &apiErr) && apiErr.Code == kannon.CodeNoRecipientsAccepted {
  for _, e := range apiErr.Errors {
    log.Printf("%s: %s", e.Pointer, e.Code)
  }
}
```

### MailSender - Simple Email API

For straightforward email sending without templating, use `MailSender`. It wraps the Kannon client with a traditional mail API that supports To, CC, and BCC recipients.

```go
mailSender := kannon.NewMailSender(k)

res, err := mailSender.Send(context.Background(), kannon.SendParams{
  To:      []string{"alice@example.com", "bob@example.com"},
  CC:      []string{"manager@example.com"},
  BCC:     []string{"archive@example.com"},
  Subject: "Monthly Report",
  Content: "<h1>Report</h1><p>See attachment.</p>",
  Attachments: []kannon.Attachment{
    {Filename: "report.pdf", Content: pdfBytes},
  },
})
```

- CC recipients are visible in the email headers to all recipients; BCC recipients receive the email but never appear in the headers.
- Set `ScheduledTime` (a `*time.Time`) to schedule for future delivery.
- Mail sent through `MailSender` is **never tracked**: opens and links are both forced to `off`, and no `List-Unsubscribe` header is emitted. A plain mail client sends mail, it does not measure it. Use the Kannon client directly for campaign mail, where those are decisions to make.
- The returned `SendResult` carries `BatchID`, `ScheduledTime` and the intake counts (`AcceptedCount`, `RejectedCount`, `RejectedRecipients`).
