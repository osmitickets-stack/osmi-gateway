// internal/server/server.go
package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/franciscozamorau/osmi-gateway/internal/cache"
	"github.com/franciscozamorau/osmi-gateway/internal/config"
	gatewayGrpc "github.com/franciscozamorau/osmi-gateway/internal/grpc"
	"github.com/franciscozamorau/osmi-gateway/internal/handlers/health"
	"github.com/franciscozamorau/osmi-gateway/internal/handlers/webhook"
	"github.com/franciscozamorau/osmi-gateway/internal/middleware"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/franciscozamorau/osmi-protobuf/gen/pb"
)

type Server struct {
	config      *config.Config
	grpcConn    *gatewayGrpc.ClientConnection
	httpServer  *http.Server
	redisClient *cache.RedisClient
}

func NewServer(cfg *config.Config) (*Server, error) {
	if cfg.JWTSecret == "" {
		log.Fatal("❌ JWT_SECRET_KEY is required in .env file")
	}

	// Inicializar Redis (opcional, para blacklist)
	var redisClient *cache.RedisClient
	if cfg.RedisURL != "" {
		var err error
		redisClient, err = cache.NewRedisClient(cfg.RedisURL, cfg.RedisPassword, cfg.RedisDB)
		if err != nil {
			log.Printf("⚠️ Redis not available, blacklist disabled: %v", err)
		} else {
			log.Println("✅ Redis connected for token blacklist")
		}
	}

	grpcConn, err := gatewayGrpc.NewClientConnection(cfg)
	if err != nil {
		return nil, err
	}

	mux := runtime.NewServeMux()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	err = pb.RegisterOsmiServiceHandlerFromEndpoint(
		context.Background(),
		mux,
		cfg.GRPCServerAddr,
		opts,
	)
	if err != nil {
		grpcConn.Close()
		return nil, err
	}

	var handler http.Handler = mux

	handler = middleware.Recovery(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.Logging(handler)
	handler = middleware.RateLimit(handler)
	handler = middleware.AuthExcludingPaths(handler, []string{
		"/v1/auth/login",
		"/health",
		"/v1/auth/refresh",
		"/v1/events",
		"/v1/events/",
		"/v1/ticket-types",    // ← tipos de boleto públicos
		"/v1/ticket-types/",   // ← Por si acaso
		"/v1/customers",       // ← Permitir crear customer sin auth
		"/v1/tickets/reserve", // ← Reserva pública
		"/v1/orders",          // ← Órdenes públicas (invitado)
		"/v1/payments",        // ← Pagos públicos (invitado)
		"/v1/payments/intent",
		"/v1/webhooks/stripe", // ← Webhook de Stripe
		"/v1/users",           // ← Permitir registro de usuarios sin auth
	}, cfg.JWTSecret, redisClient)
	handler = middleware.CORS(handler)

	mainMux := http.NewServeMux()
	mainMux.Handle("/", handler)
	mainMux.HandleFunc("/health", health.HealthHandler)
	mainMux.HandleFunc("/v1/webhooks/stripe", webhook.StripeWebhookHandler(grpcConn.GetConnection()))

	httpServer := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      mainMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		config:      cfg,
		grpcConn:    grpcConn,
		httpServer:  httpServer,
		redisClient: redisClient,
	}, nil
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	if s.redisClient != nil {
		s.redisClient.Close()
	}
	if s.grpcConn != nil {
		s.grpcConn.Close()
	}
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
