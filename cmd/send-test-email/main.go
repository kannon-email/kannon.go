// Command send-test-email sends a real test email through Kannon.
//
// It is a manual smoke test, ported from the kannon.js
// scripts/send-test-email.ts script. Configuration comes from the
// environment:
//
//	KANNON_DOMAIN    sender domain registered on Kannon (required)
//	KANNON_KEY       API key for the domain (required)
//	KANNON_EMAIL     sender email address (required)
//	KANNON_TO        comma-separated recipient addresses (required)
//	KANNON_ENDPOINT  API endpoint (default https://grpc.kannon.email)
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	kannon "github.com/kannon-email/kannon.go"
)

const defaultEndpoint = "https://grpc.kannon.email"

func main() {
	domain := requireEnv("KANNON_DOMAIN")
	key := requireEnv("KANNON_KEY")
	email := requireEnv("KANNON_EMAIL")
	to := splitRecipients(requireEnv("KANNON_TO"))

	endpoint := os.Getenv("KANNON_ENDPOINT")
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	if len(to) == 0 {
		fmt.Fprintln(os.Stderr, "KANNON_TO contains no recipient addresses")
		os.Exit(1)
	}

	fmt.Println("Domain:", domain)
	fmt.Println("Sender:", email)
	fmt.Println("Endpoint:", endpoint)
	fmt.Println("To:", strings.Join(to, ", "))

	sender := kannon.Sender{Alias: "Kannon Test", Email: email}
	client := kannon.NewKannon(domain, key, sender, endpoint)

	recipients := make([]kannon.Recipient, 0, len(to))
	for _, addr := range to {
		recipients = append(recipients, kannon.Recipient{Email: addr})
	}

	body := fmt.Sprintf(
		"<h1>Test email from kannon.go</h1>\n<p>Sent at %s</p>",
		time.Now().UTC().Format(time.RFC3339),
	)

	result, err := client.SendEmail(
		context.Background(),
		recipients,
		"Kannon Test Email",
		body,
		kannon.WithHeaders(kannon.Headers{To: to}),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to send email:", err)
		os.Exit(1)
	}

	fmt.Println("Email sent successfully!")
	fmt.Println("Message ID:", result.MessageID)
	fmt.Println("Accepted:", result.AcceptedCount)

	if result.RejectedCount > 0 {
		fmt.Println("Rejected:", result.RejectedCount)
		for _, rejected := range result.RejectedRecipients {
			fmt.Printf("  - %s: %s\n", rejected.Email, rejected.Reason)
		}
	}
}

// requireEnv returns the value of the named environment variable, exiting
// with a clear message when it is missing or empty.
func requireEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		fmt.Fprintf(os.Stderr, "Missing required environment variable: %s\n", name)
		os.Exit(1)
	}
	return value
}

// splitRecipients splits a comma-separated address list, trimming whitespace
// and dropping empty entries.
func splitRecipients(raw string) []string {
	parts := strings.Split(raw, ",")
	to := make([]string, 0, len(parts))
	for _, part := range parts {
		if addr := strings.TrimSpace(part); addr != "" {
			to = append(to, addr)
		}
	}
	return to
}
