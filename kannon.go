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

func (k *Kannon) SendEmail(ctx context.Context, to []Recipient, subject, body string) (MessageResult, error) {
	recipients := mapRecipients(to)

	req := connect.NewRequest(&apiv1.SendHTMLReq{
		Sender:        k.sender.toProto(),
		Subject:       subject,
		Html:          body,
		Recipients:    recipients,
		ScheduledTime: timestamppb.Now(),
		Attachments:   []*apiv1.Attachment{},
		GlobalFields:  map[string]string{},
	})

	req.Header().Set("Authorization", k.getAuthHeader())

	res, err := k.cli.SendHTML(ctx, req)

	if err != nil {
		return MessageResult{}, err
	}

	return MessageResult{
		MessageID:     res.Msg.MessageId,
		TemplateID:    res.Msg.TemplateId,
		ScheduledTime: res.Msg.ScheduledTime.AsTime(),
	}, nil
}

func (k *Kannon) SendTemplate(ctx context.Context, to []Recipient, subject, templateID string) (MessageResult, error) {
	recipients := mapRecipients(to)

	req := connect.NewRequest(&apiv1.SendTemplateReq{
		Sender:        k.sender.toProto(),
		Subject:       subject,
		TemplateId:    templateID,
		Recipients:    recipients,
		ScheduledTime: timestamppb.Now(),
		Attachments:   []*apiv1.Attachment{},
		GlobalFields:  map[string]string{},
	})

	req.Header().Set("Authorization", k.getAuthHeader())

	res, err := k.cli.SendTemplate(ctx, req)

	if err != nil {
		return MessageResult{}, err
	}

	return MessageResult{
		MessageID:     res.Msg.MessageId,
		TemplateID:    res.Msg.TemplateId,
		ScheduledTime: res.Msg.ScheduledTime.AsTime(),
	}, nil
}

func (k *Kannon) getAuthHeader() string {
	token := base64.StdEncoding.EncodeToString([]byte(k.domain + ":" + k.key))
	return "Basic " + token
}

func mapRecipients(recipients []Recipient) []*types.Recipient {
	res := make([]*types.Recipient, 0, len(recipients))
	for _, r := range recipients {
		res = append(res, &types.Recipient{
			Email:  r.Email,
			Fields: r.Fields,
		})
	}
	return res
}

type MessageResult struct {
	MessageID     string
	TemplateID    string
	ScheduledTime time.Time
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
}

type Fields map[string]string
