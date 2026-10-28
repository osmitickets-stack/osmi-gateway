package server

import (
	"strings"
	"testing"
)

func TestTrustedIncomingHeaderMatcher(t *testing.T) {
	for _, header := range []string{
		"Grpc-Metadata-User-Id",
		"grpc-metadata-user-id",
		"GRPC-METADATA-USER-ID",
	} {
		t.Run(header, func(t *testing.T) {
			mapped, ok := trustedIncomingHeaderMatcher(header)

			if ok {
				t.Fatalf(
					"untrusted identity header accepted as %q",
					mapped,
				)
			}
		})
	}

	mapped, ok := trustedIncomingHeaderMatcher(
		"Grpc-Metadata-Request-Id",
	)

	if !ok || !strings.EqualFold(mapped, "request-id") {
		t.Fatalf(
			"expected legitimate metadata to be preserved, got %q, %v",
			mapped,
			ok,
		)
	}
}
