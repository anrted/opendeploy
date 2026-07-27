package recovery

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HTTPMiddleware recovers from panics in HTTP handlers.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered in HTTP handler",
					"error", err,
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
				slog.Error("panic recovered in gRPC unary handler",
					"error", r,
					"stack", string(debug.Stack()),
					"method", info.FullMethod,
				)
				err = status.Error(codes.Internal, "internal agent error")
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
				slog.Error("panic recovered in gRPC stream handler",
					"error", r,
					"stack", string(debug.Stack()),
					"method", info.FullMethod,
				)
				err = status.Error(codes.Internal, "internal agent error")
			}
		}()
		return handler(srv, ss)
	}
}
