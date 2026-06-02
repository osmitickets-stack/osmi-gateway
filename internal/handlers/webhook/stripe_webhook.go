package webhook

import (
	"io"
	"log"
	"net/http"

	pb "github.com/franciscozamorau/osmi-protobuf/gen/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func StripeWebhookHandler(conn *grpc.ClientConn) http.HandlerFunc {
	client := pb.NewOsmiServiceClient(conn)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		payload, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("❌ Error reading webhook body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		signatureHeader := r.Header.Get("Stripe-Signature")
		if signatureHeader == "" {
			log.Printf("❌ Missing Stripe-Signature header")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Llamar al server gRPC con los datos crudos
		ctx := metadata.NewOutgoingContext(r.Context(), metadata.Pairs())
		_, err = client.HandleWebhook(ctx, &pb.WebhookRequest{
			Payload:         string(payload),
			SignatureHeader: signatureHeader,
		})

		if err != nil {
			log.Printf("❌ Webhook error: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"webhook processing failed"}`))
			return
		}

		log.Printf("✅ Webhook processed successfully")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}
}
