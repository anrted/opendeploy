package controlplane

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

type IdentityVerifier interface {
	VerifyCertificate(context.Context, string, string) (bool, error)
}

type HeartbeatHandler interface {
	ControlPlaneConnected(context.Context, string, *agentv1.AgentHello) error
	ControlPlaneHeartbeat(context.Context, string, string, *agentv1.AgentHeartbeat) error
	ControlPlaneEvent(context.Context, string, *agentv1.AgentEvent) error
	ControlPlaneTaskProgress(context.Context, string, *agentv1.TaskProgress) error
}

type Server struct {
	agentv1.UnimplementedControlPlaneServer
	manager   *Manager
	verifier  IdentityVerifier
	heartbeat HeartbeatHandler
}

func NewServer(manager *Manager, verifier IdentityVerifier, heartbeat HeartbeatHandler) *Server {
	return &Server{manager: manager, verifier: verifier, heartbeat: heartbeat}
}

// Connect owns the full duplex stream lifecycle. Its branches correspond to the
// finite set of protocol message types and intentionally remain co-located.
func (s *Server) Connect(stream agentv1.ControlPlane_ConnectServer) error { //nolint:gocyclo
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	hello := first.GetHello()
	if hello == nil || hello.GetServerId() == "" || hello.GetCertificateFingerprint() == "" {
		return fmt.Errorf("the first control-plane message must authenticate the agent")
	}
	peerInfo, ok := peer.FromContext(stream.Context())
	tlsInfo, tlsOK := peerInfo.AuthInfo.(credentials.TLSInfo)
	if !ok || !tlsOK || len(tlsInfo.State.PeerCertificates) == 0 {
		return fmt.Errorf("agent client certificate is required")
	}
	fingerprint := sha256.Sum256(tlsInfo.State.PeerCertificates[0].Raw)
	if !strings.EqualFold(hex.EncodeToString(fingerprint[:]), hello.GetCertificateFingerprint()) {
		return fmt.Errorf("agent certificate fingerprint does not match the authenticated peer")
	}
	valid, err := s.verifier.VerifyCertificate(stream.Context(), hello.GetServerId(), hello.GetCertificateFingerprint())
	if err != nil || !valid {
		return fmt.Errorf("agent identity rejected")
	}
	session, err := s.manager.register(hello)
	if err != nil {
		return err
	}
	defer s.manager.unregister(session)
	if s.heartbeat != nil {
		if err := s.heartbeat.ControlPlaneConnected(stream.Context(), session.serverID, hello); err != nil {
			return err
		}
	}
	if err := stream.Send(&agentv1.CoreMessage{Body: &agentv1.CoreMessage_Welcome{Welcome: &agentv1.StreamWelcome{
		ConnectionId: session.id, ProtocolVersion: ProtocolVersion,
		HeartbeatIntervalSeconds: 15, ServerTimeUnixMs: time.Now().UnixMilli(),
	}}}); err != nil {
		return err
	}
	sendErr := make(chan error, 1)
	go func() {
		for {
			select {
			case message := <-session.send:
				if err := stream.Send(message); err != nil {
					sendErr <- err
					return
				}
			case <-session.done:
				sendErr <- nil
				return
			case <-stream.Context().Done():
				sendErr <- stream.Context().Err()
				return
			}
		}
	}()
	for {
		message, recvErr := stream.Recv()
		if recvErr == io.EOF {
			return nil
		}
		if recvErr != nil {
			return recvErr
		}
		switch body := message.Body.(type) {
		case *agentv1.AgentMessage_CommandResult:
			session.resolve(body.CommandResult)
		case *agentv1.AgentMessage_Heartbeat:
			if s.heartbeat != nil {
				if err := s.heartbeat.ControlPlaneHeartbeat(stream.Context(), session.serverID, session.agentVersion, body.Heartbeat); err != nil {
					return err
				}
			}
		case *agentv1.AgentMessage_Event:
			if s.heartbeat != nil {
				if err := s.heartbeat.ControlPlaneEvent(stream.Context(), session.serverID, body.Event); err != nil {
					return err
				}
			}
		case *agentv1.AgentMessage_TaskProgress:
			if s.heartbeat != nil {
				if err := s.heartbeat.ControlPlaneTaskProgress(stream.Context(), session.serverID, body.TaskProgress); err != nil {
					return err
				}
			}
		case *agentv1.AgentMessage_Chunk:
			session.resolveChunk(body.Chunk)
		case *agentv1.AgentMessage_Pong:
			// Receiving any message proves liveness; heartbeat owns persisted metrics.
		case *agentv1.AgentMessage_Hello:
			return fmt.Errorf("agent hello may only be sent once")
		}
		select {
		case err := <-sendErr:
			return err
		default:
		}
	}
}
