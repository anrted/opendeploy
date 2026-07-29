package recovery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HTTPMiddleware recovers from panics in HTTP handlers.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				appErr := apperrors.Internal("unexpected server error", nil)
				slog.Error("panic recovered in HTTP handler",
					"error_id", appErr.ErrorID,
					"error", err,
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
				)
				apperrors.WriteHTTP(w, appErr)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// GRPCUnaryInterceptor recovers from panics in gRPC unary handlers.
func GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				errorIDBytes := make([]byte, 8)
				rand.Read(errorIDBytes)
				encodedID := hex.EncodeToString(errorIDBytes)
				slog.Error("panic recovered in gRPC unary handler",
					"error_id", encodedID,
					"error", r,
					"stack", string(debug.Stack()),
					"method", info.FullMethod,
				)
				err = status.Error(codes.Internal, "internal agent error: "+encodedID)
			}
		}()
		return handler(ctx, req)
	}
}

// GRPCStreamInterceptor recovers from panics in gRPC stream handlers.
func GRPCStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if r := recover(); r != nil {
				errorIDBytes := make([]byte, 8)
				rand.Read(errorIDBytes)
				encodedID := hex.EncodeToString(errorIDBytes)
				slog.Error("panic recovered in gRPC stream handler",
					"error_id", encodedID,
					"error", r,
					"stack", string(debug.Stack()),
					"method", info.FullMethod,
				)
				err = status.Error(codes.Internal, "internal agent error: "+encodedID)
			}
		}()
		return handler(srv, ss)
	}
}
