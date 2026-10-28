package grpc

import (
	"context"
	"testing"

	"github.com/osmitickets-stack/osmi-gateway/internal/middleware"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestAuthInterceptorReplacesUntrustedIdentity(t *testing.T) {
	tests := []struct {
		name          string
		authenticated string
		injected      string
		expected      string
	}{
		{
			name:          "authenticated user",
			authenticated: "user-a",
			expected:      "user-a",
		},
		{
			name:          "injected identity is replaced",
			authenticated: "user-a",
			injected:      "user-b",
			expected:      "user-a",
		},
		{
			name:     "unauthenticated request cannot inject identity",
			injected: "user-b",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			if tc.authenticated != "" {
				ctx = context.WithValue(
					ctx,
					middleware.UserIDKey,
					tc.authenticated,
				)
			}

			if tc.injected != "" {
				ctx = metadata.NewOutgoingContext(
					ctx,
					metadata.Pairs("user-id", tc.injected),
				)
			}

			interceptor := NewAuthInterceptor("test-secret")
			called := false

			err := interceptor.UnaryClientInterceptor(
				ctx,
				"/test.Service/Method",
				nil,
				nil,
				nil,
				func(
					callCtx context.Context,
					method string,
					req, reply interface{},
					cc *grpcgo.ClientConn,
					opts ...grpcgo.CallOption,
				) error {
					called = true

					md, _ := metadata.FromOutgoingContext(callCtx)
					ids := md.Get("user-id")

					if tc.expected == "" {
						if len(ids) != 0 {
							t.Fatalf("unexpected identity: %v", ids)
						}
						return nil
					}

					if len(ids) != 1 || ids[0] != tc.expected {
						t.Fatalf(
							"expected only %q, got %v",
							tc.expected,
							ids,
						)
					}

					return nil
				},
			)

			if err != nil {
				t.Fatalf("interceptor returned error: %v", err)
			}

			if !called {
				t.Fatal("gRPC invoker was not called")
			}
		})
	}
}
