// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: kannon/mailer/apiv1/mailerapiv1.proto

package apiv1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	apiv1 "github.com/kannon-email/kannon.go/internal/proto/kannon/mailer/apiv1"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect.IsAtLeastVersion1_13_0

const (
	// MailerName is the fully-qualified name of the Mailer service.
	MailerName = "pkg.kannon.mailer.apiv1.Mailer"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// MailerSendHTMLProcedure is the fully-qualified name of the Mailer's SendHTML RPC.
	MailerSendHTMLProcedure = "/pkg.kannon.mailer.apiv1.Mailer/SendHTML"
	// MailerSendTemplateProcedure is the fully-qualified name of the Mailer's SendTemplate RPC.
	MailerSendTemplateProcedure = "/pkg.kannon.mailer.apiv1.Mailer/SendTemplate"
)

// These variables are the protoreflect.Descriptor objects for the RPCs defined in this package.
var (
	mailerServiceDescriptor            = apiv1.File_kannon_mailer_apiv1_mailerapiv1_proto.Services().ByName("Mailer")
	mailerSendHTMLMethodDescriptor     = mailerServiceDescriptor.Methods().ByName("SendHTML")
	mailerSendTemplateMethodDescriptor = mailerServiceDescriptor.Methods().ByName("SendTemplate")
)

// MailerClient is a client for the pkg.kannon.mailer.apiv1.Mailer service.
type MailerClient interface {
	SendHTML(context.Context, *connect.Request[apiv1.SendHTMLReq]) (*connect.Response[apiv1.SendRes], error)
	SendTemplate(context.Context, *connect.Request[apiv1.SendTemplateReq]) (*connect.Response[apiv1.SendRes], error)
}

// NewMailerClient constructs a client for the pkg.kannon.mailer.apiv1.Mailer service. By default,
// it uses the Connect protocol with the binary Protobuf Codec, asks for gzipped responses, and
// sends uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC()
// or connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewMailerClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) MailerClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &mailerClient{
		sendHTML: connect.NewClient[apiv1.SendHTMLReq, apiv1.SendRes](
			httpClient,
			baseURL+MailerSendHTMLProcedure,
			connect.WithSchema(mailerSendHTMLMethodDescriptor),
			connect.WithClientOptions(opts...),
		),
		sendTemplate: connect.NewClient[apiv1.SendTemplateReq, apiv1.SendRes](
			httpClient,
			baseURL+MailerSendTemplateProcedure,
			connect.WithSchema(mailerSendTemplateMethodDescriptor),
			connect.WithClientOptions(opts...),
		),
	}
}

// mailerClient implements MailerClient.
type mailerClient struct {
	sendHTML     *connect.Client[apiv1.SendHTMLReq, apiv1.SendRes]
	sendTemplate *connect.Client[apiv1.SendTemplateReq, apiv1.SendRes]
}

// SendHTML calls pkg.kannon.mailer.apiv1.Mailer.SendHTML.
func (c *mailerClient) SendHTML(ctx context.Context, req *connect.Request[apiv1.SendHTMLReq]) (*connect.Response[apiv1.SendRes], error) {
	return c.sendHTML.CallUnary(ctx, req)
}

// SendTemplate calls pkg.kannon.mailer.apiv1.Mailer.SendTemplate.
func (c *mailerClient) SendTemplate(ctx context.Context, req *connect.Request[apiv1.SendTemplateReq]) (*connect.Response[apiv1.SendRes], error) {
	return c.sendTemplate.CallUnary(ctx, req)
}

// MailerHandler is an implementation of the pkg.kannon.mailer.apiv1.Mailer service.
type MailerHandler interface {
	SendHTML(context.Context, *connect.Request[apiv1.SendHTMLReq]) (*connect.Response[apiv1.SendRes], error)
	SendTemplate(context.Context, *connect.Request[apiv1.SendTemplateReq]) (*connect.Response[apiv1.SendRes], error)
}

// NewMailerHandler builds an HTTP handler from the service implementation. It returns the path on
// which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewMailerHandler(svc MailerHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	mailerSendHTMLHandler := connect.NewUnaryHandler(
		MailerSendHTMLProcedure,
		svc.SendHTML,
		connect.WithSchema(mailerSendHTMLMethodDescriptor),
		connect.WithHandlerOptions(opts...),
	)
	mailerSendTemplateHandler := connect.NewUnaryHandler(
		MailerSendTemplateProcedure,
		svc.SendTemplate,
		connect.WithSchema(mailerSendTemplateMethodDescriptor),
		connect.WithHandlerOptions(opts...),
	)
	return "/pkg.kannon.mailer.apiv1.Mailer/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case MailerSendHTMLProcedure:
			mailerSendHTMLHandler.ServeHTTP(w, r)
		case MailerSendTemplateProcedure:
			mailerSendTemplateHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedMailerHandler returns CodeUnimplemented from all methods.
type UnimplementedMailerHandler struct{}

func (UnimplementedMailerHandler) SendHTML(context.Context, *connect.Request[apiv1.SendHTMLReq]) (*connect.Response[apiv1.SendRes], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("pkg.kannon.mailer.apiv1.Mailer.SendHTML is not implemented"))
}

func (UnimplementedMailerHandler) SendTemplate(context.Context, *connect.Request[apiv1.SendTemplateReq]) (*connect.Response[apiv1.SendRes], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("pkg.kannon.mailer.apiv1.Mailer.SendTemplate is not implemented"))
}
