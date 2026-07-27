package recovery

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGRPCRecoveryDoesNotLeakPanicValue(t *testing.T) {
	interceptor := GRPCUnaryInterceptor()
	_, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/agent.v1.AgentService/Test"},
		func(context.Context, any) (any, error) {
			panic("secret internal value")
		},
	)
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %s, want Internal", status.Code(err))
	}
	if strings.Contains(status.Convert(err).Message(), "secret") {
		t.Fatalf("panic value leaked to client: %v", err)
	}
}
