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
  "<YOU KANNON API HOST>",
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
  recipents,
  "This is an email from kannon.go",
  templateID,
);
```
