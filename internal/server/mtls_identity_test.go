package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"google.golang.org/protobuf/encoding/protojson"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/osmitickets-stack/osmi-gateway/internal/config"
	gatewaygrpc "github.com/osmitickets-stack/osmi-gateway/internal/grpc"
	"github.com/osmitickets-stack/osmi-gateway/internal/middleware"
	"github.com/osmitickets-stack/osmi-gateway/internal/testutil/grpcpki"
	"github.com/osmitickets-stack/osmi-gateway/internal/transportsecurity"
	pb "github.com/osmitickets-stack/osmi-protobuf/gen/pb"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type secureValidationProbe struct {
	pb.UnimplementedOsmiServiceServer
	calls atomic.Int32
}

func (p *secureValidationProbe) ValidateTicketCredential(ctx context.Context, req *pb.ValidateTicketCredentialRequest) (*pb.ValidateTicketCredentialResponse, error) {
	p.calls.Add(1)
	md, _ := metadata.FromIncomingContext(ctx)
	ids := md.Get("user-id")
	if len(ids) != 1 || (ids[0] != "11111111-1111-4111-8111-111111111111" && ids[0] != "22222222-2222-4222-8222-222222222222") {
		return nil, status.Error(codes.Unauthenticated, "invalid gateway identity assertion")
	}
	auth := md.Get("authorization")
	if len(auth) != 1 || !strings.HasPrefix(auth[0], "Bearer ") {
		return nil, status.Error(codes.Unauthenticated, "validated token missing")
	}
	validated, err := middleware.AuthenticatedUserID(strings.TrimPrefix(auth[0], "Bearer "), "0123456789abcdef0123456789abcdef")
	if err != nil || validated != ids[0] {
		return nil, status.Error(codes.Unauthenticated, "token and identity differ")
	}
	for _, key := range []string{"user_id", "x-user-id", "roles", "is-superuser"} {
		if len(md.Get(key)) > 0 {
			return nil, status.Error(codes.Unauthenticated, "untrusted identity metadata forwarded")
		}
	}
	pinfo, _ := peer.FromContext(ctx)
	tlsInfo, ok := pinfo.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.VerifiedChains) == 0 || tlsInfo.State.PeerCertificates[0].DNSNames[0] != "osmi-gateway" {
		return nil, status.Error(codes.Unauthenticated, "untrusted transport")
	}
	return &pb.ValidateTicketCredentialResponse{Authentic: true, Result: "VALID", TicketId: ids[0]}, nil
}

func TestHTTPIdentityThroughMutualTLS(t *testing.T) {
	ca := grpcpki.New(t)
	serverID := ca.Issue(t, "osmi-server", x509.ExtKeyUsageServerAuth, false)
	gatewayID := ca.Issue(t, "osmi-gateway", x509.ExtKeyUsageClientAuth, false)
	certificate, err := tls.LoadX509KeyPair(serverID.CertFile, serverID.KeyFile)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(ca.File)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(data)
	backend := grpcgo.NewServer(grpcgo.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert})))
	probe := &secureValidationProbe{}
	pb.RegisterOsmiServiceServer(backend, probe)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go backend.Serve(lis)
	t.Cleanup(backend.Stop)
	cfg := &config.Config{GRPCServerAddr: lis.Addr().String(), JWTSecret: "0123456789abcdef0123456789abcdef", GRPCTLS: transportsecurity.Config{CAFile: ca.File, CertFile: gatewayID.CertFile, KeyFile: gatewayID.KeyFile, ServerName: "osmi-server"}}
	conn, err := gatewaygrpc.NewClientConnection(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	mux := runtime.NewServeMux(runtime.WithIncomingHeaderMatcher(trustedIncomingHeaderMatcher))
	if err := pb.RegisterOsmiServiceHandlerClient(context.Background(), mux, pb.NewOsmiServiceClient(conn.GetConnection())); err != nil {
		t.Fatal(err)
	}
	handler := middleware.Auth(mux, cfg.JWTSecret, nil)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": "11111111-1111-4111-8111-111111111111", "exp": time.Now().Add(time.Minute).Unix()}).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, token string
		code        int
	}{{"legitimate user with spoofed headers", token, 200}, {"anonymous spoof", "", 401}, {"invalid token spoof", "forged", 401}} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/tickets/validate", strings.NewReader(`{"credential":"qr","event_id":"22222222-2222-4222-8222-222222222222"}`))
			ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
			defer cancel()
			req = req.WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			for _, key := range []string{"Grpc-Metadata-User-Id", "Grpc-Metadata-User_Id", "Grpc-Metadata-X-User-Id", "Grpc-Metadata-Roles", "Grpc-Metadata-Is-Superuser", "Grpc-Metadata-Authorization"} {
				req.Header.Set(key, "attacker")
			}
			before := probe.calls.Load()
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Code != tc.code {
				t.Fatalf("HTTP %d: %s", out.Code, out.Body.String())
			}
			if tc.code != 200 && probe.calls.Load() != before {
				t.Fatal("invalid HTTP caller reached server")
			}
		})
	}
	t.Run("concurrent requests preserve separate identities", func(t *testing.T) {
		var wg sync.WaitGroup
		failures := make(chan error, 16)
		start := make(chan struct{})
		for i := 0; i < 16; i++ {
			user := "11111111-1111-4111-8111-111111111111"
			if i%2 == 1 {
				user = "22222222-2222-4222-8222-222222222222"
			}
			signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": user, "exp": time.Now().Add(time.Minute).Unix()}).SignedString([]byte(cfg.JWTSecret))
			if err != nil {
				t.Fatal(err)
			}
			wg.Add(1)
			go func(expected, token string) {
				defer wg.Done()
				<-start
				req := httptest.NewRequest(http.MethodPost, "/v1/tickets/validate", strings.NewReader("{\"credential\":\"qr\",\"event_id\":\"22222222-2222-4222-8222-222222222222\"}"))
				ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
				defer cancel()
				req = req.WithContext(ctx)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Grpc-Metadata-User-Id", "attacker")
				out := httptest.NewRecorder()
				handler.ServeHTTP(out, req)
				result := &pb.ValidateTicketCredentialResponse{}
				if err := protojson.Unmarshal(out.Body.Bytes(), result); err != nil || out.Code != 200 || result.TicketId != expected {
					failures <- fmt.Errorf("identity isolation failed: expected=%s status=%d result=%v err=%v", expected, out.Code, result, err)
				}
			}(user, signed)
		}
		close(start)
		wg.Wait()
		close(failures)
		for err := range failures {
			t.Error(err)
		}
	})

}

func TestGatewayRequiresTLSConfiguration(t *testing.T) {
	if _, err := gatewaygrpc.NewClientConnection(&config.Config{}); err == nil {
		t.Fatal("missing TLS configuration accepted")
	}
}
