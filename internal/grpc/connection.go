package grpc

import (
	"log"
	"time"

	"github.com/osmitickets-stack/osmi-gateway/internal/config"
	"github.com/osmitickets-stack/osmi-gateway/internal/transportsecurity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type ClientConnection struct {
	conn *grpc.ClientConn
	cfg  *config.Config
}

func NewClientConnection(cfg *config.Config) (*ClientConnection, error) {
	transportCredentials, err := transportsecurity.LoadClientCredentials(cfg.GRPCTLS)
	if err != nil {
		return nil, err
	}
	keepaliveParams := keepalive.ClientParameters{
		Time:                60 * time.Second,
		Timeout:             20 * time.Second,
		PermitWithoutStream: true,
	}

	authInterceptor := NewAuthInterceptor(cfg.JWTSecret)

	conn, err := grpc.NewClient(
		cfg.GRPCServerAddr,
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithKeepaliveParams(keepaliveParams),
		grpc.WithUnaryInterceptor(authInterceptor.UnaryClientInterceptor),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1024*1024*10),
			grpc.MaxCallSendMsgSize(1024*1024*10),
		),
		grpc.WithConnectParams(grpc.ConnectParams{
			MinConnectTimeout: 5 * time.Second,
		}),
	)
	if err != nil {
		return nil, err
	}

	log.Printf("Cliente gRPC con mTLS configurado para %s", cfg.GRPCServerAddr)

	return &ClientConnection{
		conn: conn,
		cfg:  cfg,
	}, nil
}

func (c *ClientConnection) GetConnection() *grpc.ClientConn {
	return c.conn
}

func (c *ClientConnection) Close() error {
	if c.conn != nil {
		log.Printf("Cerrando conexión gRPC con %s", c.cfg.GRPCServerAddr)
		return c.conn.Close()
	}
	return nil
}
