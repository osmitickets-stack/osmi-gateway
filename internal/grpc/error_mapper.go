// internal/grpc/error_mapper.go
package grpc

import (
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MapGRPCErrorToHTTP convierte errores gRPC a códigos HTTP
func MapGRPCErrorToHTTP(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}

	st, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError, "internal_error"
	}

	switch st.Code() {
	case codes.OK:
		return http.StatusOK, ""
	case codes.Canceled:
		return http.StatusRequestTimeout, "request_canceled"
	case codes.Unknown:
		return http.StatusInternalServerError, "unknown_error"
	case codes.InvalidArgument:
		return http.StatusBadRequest, "invalid_argument"
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout, "deadline_exceeded"
	case codes.NotFound:
		return http.StatusNotFound, "not_found"
	case codes.AlreadyExists:
		return http.StatusConflict, "already_exists"
	case codes.PermissionDenied:
		return http.StatusForbidden, "permission_denied"
	case codes.Unauthenticated:
		return http.StatusUnauthorized, "unauthenticated"
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests, "resource_exhausted"
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed, "failed_precondition"
	case codes.Aborted:
		return http.StatusConflict, "aborted"
	case codes.OutOfRange:
		return http.StatusBadRequest, "out_of_range"
	case codes.Unimplemented:
		return http.StatusNotImplemented, "unimplemented"
	case codes.Internal:
		return http.StatusInternalServerError, "internal_error"
	case codes.Unavailable:
		return http.StatusServiceUnavailable, "unavailable"
	case codes.DataLoss:
		return http.StatusInternalServerError, "data_loss"
	default:
		return http.StatusInternalServerError, "unknown_grpc_error"
	}
}
