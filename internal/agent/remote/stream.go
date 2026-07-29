package remote

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"github.com/anrted/opendeploy/internal/agent/stats"
	"github.com/anrted/opendeploy/internal/platform/config"
	"github.com/anrted/opendeploy/pkg/version"
	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const streamProtocolVersion uint32 = 1

type CommandHandler func(context.Context, string, []byte) ([]byte, error)
type SubscriptionHandler func(context.Context, string, []byte, func([]byte) error) error

type StreamClient struct {
	cfg       config.AgentConfig
	logger    *slog.Logger
	collector *stats.Collector
	handle    CommandHandler
	subscribe SubscriptionHandler
}

func NewStream(cfg config.AgentConfig, logger *slog.Logger, handler CommandHandler, subscribe SubscriptionHandler) (*StreamClient, error) {
	if cfg.ControlPlaneAddress == "" || cfg.ControlPlaneCAFile == "" ||
		cfg.ServerID == "" || cfg.CertificateFingerprint == "" {
		return nil, fmt.Errorf("control-plane address, CA, server ID and certificate fingerprint are required")
	}
	return &StreamClient{cfg: cfg, logger: logger, collector: stats.NewCollector(), handle: handler, subscribe: subscribe}, nil
}

func (c *StreamClient) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		started := time.Now()
		if err := c.connect(ctx); err != nil && ctx.Err() == nil {
			c.logger.WarnContext(ctx, "control-plane stream disconnected", "error", err, "retry_in", backoff)
		}
		if time.Since(started) > time.Minute {
			backoff = time.Second
		}
		delay := backoff + time.Duration(rand.Int63n(int64(backoff/2+1))) // #nosec G404 -- jitter is not security-sensitive.
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *StreamClient) connect(ctx context.Context) error {
	caPEM, err := os.ReadFile(c.cfg.ControlPlaneCAFile)
	if err != nil {
		return fmt.Errorf("load control-plane CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return fmt.Errorf("control-plane CA contains no certificates")
	}
	certificate, err := tls.LoadX509KeyPair(c.cfg.CertificateFile, c.cfg.PrivateKeyFile)
	if err != nil {
		return fmt.Errorf("load control-plane client identity: %w", err)
	}
	creds := credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12, RootCAs: roots,
		ServerName: c.cfg.ControlPlaneServerName, Certificates: []tls.Certificate{certificate},
	})
	conn, err := grpc.NewClient(c.cfg.ControlPlaneAddress,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	stream, err := agentv1.NewControlPlaneClient(conn).Connect(ctx)
	if err != nil {
		return err
	}
	outbound := make(chan *agentv1.AgentMessage, 64)
	sendErr := make(chan error, 1)
	go func() {
		for {
			select {
			case message := <-outbound:
				if err := stream.Send(message); err != nil {
					sendErr <- err
					return
				}
			case <-ctx.Done():
				sendErr <- ctx.Err()
				return
			}
		}
	}()
	outbound <- &agentv1.AgentMessage{Body: &agentv1.AgentMessage_Hello{Hello: &agentv1.AgentHello{
		ServerId: c.cfg.ServerID, CertificateFingerprint: c.cfg.CertificateFingerprint,
		ProtocolVersion: streamProtocolVersion, ApiVersion: "v1", AgentVersion: version.Version,
		Capabilities: []string{
			"dashboard", "sites", "modules", "processes", "services", "files",
			"firewall", "cron", "certificates", "logs", "packages", "tasks",
			"settings", "events", "system",
		},
	}}}
	welcome, err := stream.Recv()
	if err != nil {
		return err
	}
	if welcome.GetWelcome() == nil || welcome.GetWelcome().GetProtocolVersion() != streamProtocolVersion {
		return fmt.Errorf("control-plane protocol negotiation failed")
	}
	interval := time.Duration(welcome.GetWelcome().GetHeartbeatIntervalSeconds()) * time.Second
	go c.heartbeatLoop(ctx, interval, outbound)
	subscriptions := make(map[string]context.CancelFunc)
	for {
		message, recvErr := stream.Recv()
		if recvErr != nil {
			return recvErr
		}
		switch body := message.Body.(type) {
		case *agentv1.CoreMessage_Command:
			go c.execute(ctx, body.Command, outbound)
		case *agentv1.CoreMessage_Ping:
			outbound <- &agentv1.AgentMessage{Body: &agentv1.AgentMessage_Pong{Pong: &agentv1.Pong{SentAtUnixMs: body.Ping.GetSentAtUnixMs()}}}
		case *agentv1.CoreMessage_Subscribe:
			subscriptionCtx, cancel := context.WithCancel(ctx)
			subscriptions[body.Subscribe.GetId()] = cancel
			go c.runSubscription(subscriptionCtx, body.Subscribe, outbound)
		case *agentv1.CoreMessage_Cancel:
			if cancel := subscriptions[body.Cancel.GetId()]; cancel != nil {
				cancel()
				delete(subscriptions, body.Cancel.GetId())
			}
		}
		select {
		case err := <-sendErr:
			return err
		default:
		}
	}
}

func (c *StreamClient) runSubscription(ctx context.Context, subscription *agentv1.StreamSubscription, outbound chan<- *agentv1.AgentMessage) {
	var sequence uint64
	emit := func(data []byte) error {
		sequence++
		select {
		case outbound <- &agentv1.AgentMessage{Body: &agentv1.AgentMessage_Chunk{Chunk: &agentv1.StreamChunk{
			StreamId: subscription.GetId(), Sequence: sequence, Data: data,
		}}}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	err := c.subscribe(ctx, subscription.GetKind(), subscription.GetPayload(), emit)
	if err != nil {
		_ = emit([]byte("stream error: " + err.Error()))
	}
	select {
	case outbound <- &agentv1.AgentMessage{Body: &agentv1.AgentMessage_Chunk{Chunk: &agentv1.StreamChunk{
		StreamId: subscription.GetId(), Sequence: sequence + 1, Eof: true,
	}}}:
	case <-ctx.Done():
	}
}

func (c *StreamClient) execute(parent context.Context, command *agentv1.AgentCommand, outbound chan<- *agentv1.AgentMessage) {
	ctx := parent
	cancel := func() {}
	if command.GetDeadlineUnixMs() > 0 {
		ctx, cancel = context.WithDeadline(parent, time.UnixMilli(command.GetDeadlineUnixMs()))
	}
	defer cancel()
	if command.GetAsynchronous() {
		outbound <- &agentv1.AgentMessage{Body: &agentv1.AgentMessage_TaskProgress{TaskProgress: &agentv1.TaskProgress{
			TaskId: command.GetId(), Percent: 0, State: "running", Message: "operation started",
		}}}
	}
	payload, err := c.handle(ctx, command.GetKind(), command.GetPayload())
	result := &agentv1.CommandResult{Id: command.GetId(), Success: err == nil, Payload: payload}
	if err != nil {
		result.Error = err.Error()
	}
	if command.GetAsynchronous() {
		state := "success"
		message := "operation completed"
		if err != nil {
			state, message = "error", err.Error()
		}
		outbound <- &agentv1.AgentMessage{Body: &agentv1.AgentMessage_TaskProgress{TaskProgress: &agentv1.TaskProgress{
			TaskId: command.GetId(), Percent: 100, State: state, Message: message, Result: payload,
		}}}
	}
	select {
	case outbound <- &agentv1.AgentMessage{Body: &agentv1.AgentMessage_CommandResult{CommandResult: result}}:
	case <-parent.Done():
	}
}

func (c *StreamClient) heartbeatLoop(ctx context.Context, interval time.Duration, outbound chan<- *agentv1.AgentMessage) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		snapshot, err := c.collector.Collect()
		if err == nil {
			diskUsage := 0.0
			for _, disk := range snapshot.Disk {
				if disk.UsedPercent > diskUsage {
					diskUsage = disk.UsedPercent
				}
			}
			message := &agentv1.AgentMessage{Body: &agentv1.AgentMessage_Heartbeat{Heartbeat: &agentv1.AgentHeartbeat{
				CpuUsage: snapshot.CPU.UsagePercent, MemoryUsage: snapshot.Memory.UsedPercent,
				DiskUsage: diskUsage, NetworkRx: snapshot.Network.BytesRecv, NetworkTx: snapshot.Network.BytesSent,
				Load_1: snapshot.LoadAverage[0], Load_5: snapshot.LoadAverage[1], Load_15: snapshot.LoadAverage[2],
				Uptime: snapshot.Uptime, Health: "healthy", SentAtUnixMs: time.Now().UnixMilli(),
			}}}
			select {
			case outbound <- message:
			case <-ctx.Done():
				return
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}
