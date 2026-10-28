package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pb "github.com/osmitickets-stack/osmi-protobuf/gen/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type validationClient struct {
	pb.OsmiServiceClient
	request *pb.ValidateTicketCredentialRequest
	err     error
}

func (c *validationClient) ValidateTicketCredential(_ context.Context, req *pb.ValidateTicketCredentialRequest, _ ...grpc.CallOption) (*pb.ValidateTicketCredentialResponse, error) {
	c.request = req
	return &pb.ValidateTicketCredentialResponse{Authentic: true}, c.err
}

func TestValidateEventHTTPContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code int
	}{
		{"success", nil, http.StatusOK},
		{"missing event", status.Error(codes.InvalidArgument, "event_id is required"), http.StatusBadRequest},
		{"unauthenticated", status.Error(codes.Unauthenticated, "authentication required"), http.StatusUnauthorized},
		{"wrong event", status.Error(codes.PermissionDenied, "ticket does not belong to this event"), http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &validationClient{err: tc.err}
			mux := runtime.NewServeMux()
			if err := pb.RegisterOsmiServiceHandlerClient(context.Background(), mux, client); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/tickets/validate", strings.NewReader(`{"credential":"signed-qr","event_id":"22222222-2222-4222-8222-222222222222"}`))
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			mux.ServeHTTP(out, req)
			if out.Code != tc.code {
				t.Fatalf("status=%d body=%s", out.Code, out.Body.String())
			}
			if client.request == nil || client.request.GetEventId() != "22222222-2222-4222-8222-222222222222" || client.request.GetCredential() != "signed-qr" {
				t.Fatalf("request lost fields: %+v", client.request)
			}
		})
	}
}
