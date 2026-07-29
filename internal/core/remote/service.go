package remote

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
	"github.com/google/uuid"
)

const heartbeatInterval = 30

var (
	namePattern    = regexp.MustCompile(`^[\pL\pN][\pL\pN ._-]{0,79}$`)
	allowedActions = map[string]bool{
		"reconnect": true, "restart_agent": true, "stop_agent": true,
		"update_agent": true, "restart_server": true, "shutdown": true,
		"health_check": true, "refresh_information": true,
	}
)

type Service struct {
	repo *Repository
	now  func() time.Time
}

// ControlPlaneHeartbeat persists liveness received over the bidirectional
// stream. Authentication has already been performed by the stream server.
func (s *Service) ControlPlaneHeartbeat(ctx context.Context, serverID string, heartbeat *agentv1.AgentHeartbeat) error {
	req := HeartbeatRequest{
		State: "online", CPUUsage: heartbeat.GetCpuUsage(),
		MemoryUsage: heartbeat.GetMemoryUsage(), DiskUsage: heartbeat.GetDiskUsage(),
		Uptime: heartbeat.GetUptime(), RunningTasks: int(heartbeat.GetRunningTasks()),
	}
	return s.repo.Heartbeat(ctx, serverID, req, 0, s.now())
}

func (s *Service) ControlPlaneEvent(ctx context.Context, serverID string, event *agentv1.AgentEvent) error {
	return s.repo.RecordEvent(ctx, serverID, event.GetType(), string(event.GetPayload()), s.now())
}

func (s *Service) ControlPlaneTaskProgress(ctx context.Context, serverID string, progress *agentv1.TaskProgress) error {
	return s.repo.UpdateTaskProgress(ctx, serverID, progress.GetTaskId(), progress.GetState(), progress.GetMessage(), string(progress.GetResult()), s.now())
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Create(ctx context.Context, req CreateRequest, coreURL string) (*Enrollment, error) {
	req.Name = strings.TrimSpace(req.Name)
	if !namePattern.MatchString(req.Name) {
		return nil, fmt.Errorf("server name is invalid")
	}
	if req.UpdateChannel == "" {
		req.UpdateChannel = "stable"
	}
	if req.UpdateChannel != "stable" && req.UpdateChannel != "beta" && req.UpdateChannel != "nightly" {
		return nil, fmt.Errorf("update channel is invalid")
	}
	if len(req.Tags) > 20 {
		return nil, fmt.Errorf("too many tags")
	}
	now := s.now()
	server := Server{
		ID: uuid.NewString(), Name: req.Name, Description: strings.TrimSpace(req.Description),
		OS: req.OperatingSystem, Status: "pending", Tags: sanitizeTags(req.Tags),
		UpdateChannel: req.UpdateChannel, HealthStatus: "unknown", CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, &server); err != nil {
		return nil, err
	}
	enrollment, err := s.createEnrollment(ctx, server, coreURL, false)
	if err != nil {
		_ = s.repo.Delete(ctx, server.ID)
		return nil, err
	}
	return enrollment, nil
}

func (s *Service) ReissueEnrollment(ctx context.Context, serverID, coreURL string) (*Enrollment, error) {
	server, err := s.repo.Get(ctx, serverID)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, ErrServerNotFound
	}
	if server.Status != "pending" {
		return nil, ErrServerNotPending
	}
	return s.createEnrollment(ctx, *server, coreURL, true)
}

func (s *Service) createEnrollment(ctx context.Context, server Server, coreURL string, replace bool) (*Enrollment, error) {
	now := s.now()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := "odreg_" + base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	expires := now.Add(30 * time.Minute)
	save := s.repo.SaveToken
	if replace {
		save = s.repo.ReplaceToken
	}
	if err := save(ctx, uuid.NewString(), server.ID, hex.EncodeToString(hash[:]), expires, now); err != nil {
		return nil, err
	}
	codeBytes := sha256.Sum256([]byte(token))
	code := strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(codeBytes[:8]))
	code = "OD-" + code[:4] + "-" + code[4:8] + "-" + code[8:12]
	command := fmt.Sprintf("curl -fsSL https://raw.githubusercontent.com/anrted/opendeploy/main/install-agent.sh | sudo bash -s -- --server %s --token %s", strings.TrimRight(coreURL, "/"), token)
	return &Enrollment{Server: server, RegistrationToken: token, RegistrationCode: code, ExpiresAt: expires, InstallationCommand: command}, nil
}

func sanitizeTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || len(tag) > 32 || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}

func (s *Service) Register(ctx context.Context, req RegistrationRequest) (*RegistrationResponse, error) {
	if req.Token == "" || req.MachineID == "" || req.Hostname == "" || req.CSRPEM == "" {
		return nil, fmt.Errorf("token, machine_id, hostname and csr are required")
	}
	hash := sha256.Sum256([]byte(req.Token))
	now := s.now()
	serverID, err := s.repo.ConsumeToken(ctx, hex.EncodeToString(hash[:]), now)
	if err != nil {
		return nil, fmt.Errorf("registration token is invalid, expired, or already used")
	}
	cert, fingerprint, expires, err := issueClientCertificate(serverID, req.Hostname, req.CSRPEM, now)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Register(ctx, serverID, req, fingerprint, cert, expires, now); err != nil {
		return nil, err
	}
	server, err := s.repo.Get(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return &RegistrationResponse{
		ServerID: serverID, CertificatePEM: cert, Fingerprint: fingerprint,
		HeartbeatInterval: heartbeatInterval, UpdateChannel: server.UpdateChannel,
		AllowedFeatures: []string{"heartbeat", "tasks", "health", "updates"},
	}, nil
}

func issueClientCertificate(serverID, hostname, csrPEM string, now time.Time) (string, string, time.Time, error) {
	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return "", "", time.Time{}, fmt.Errorf("invalid certificate signing request")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil || csr.CheckSignature() != nil {
		return "", "", time.Time{}, fmt.Errorf("invalid certificate signing request")
	}
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", time.Time{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", time.Time{}, err
	}
	expires := now.Add(90 * 24 * time.Hour)
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: serverID, Organization: []string{"OpenDeploy Agents"}},
		DNSNames: []string{hostname}, NotBefore: now.Add(-time.Minute), NotAfter: expires,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "OpenDeploy Enrollment CA"},
		NotBefore: now.Add(-time.Minute), NotAfter: expires, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caTemplate, csr.PublicKey, caKey)
	if err != nil {
		return "", "", time.Time{}, err
	}
	sum := sha256.Sum256(der)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		hex.EncodeToString(sum[:]), expires, nil
}

func (s *Service) Heartbeat(ctx context.Context, serverID, fingerprint string, req HeartbeatRequest, latency int64) (*HeartbeatResponse, error) {
	valid, err := s.repo.VerifyCertificate(ctx, serverID, fingerprint)
	if err != nil || !valid {
		return nil, fmt.Errorf("agent certificate is invalid")
	}
	now := s.now()
	if req.State == "" {
		req.State = "online"
	}
	if err := s.repo.Heartbeat(ctx, serverID, req, latency, now); err != nil {
		return nil, err
	}
	tasks, err := s.repo.PendingTasks(ctx, serverID, now)
	if err != nil {
		return nil, err
	}
	return &HeartbeatResponse{HeartbeatInterval: heartbeatInterval, Tasks: tasks}, nil
}

func (s *Service) CreateTask(ctx context.Context, serverID, action string, payload string) (*Task, error) {
	if !allowedActions[action] {
		return nil, fmt.Errorf("unsupported server action")
	}
	if server, err := s.repo.Get(ctx, serverID); err != nil || server == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("server not found")
	}
	task := Task{ID: uuid.NewString(), ServerID: serverID, Action: action, Payload: payload, State: "pending", CreatedAt: s.now()}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *Service) SetMaintenance(ctx context.Context, serverID string, enabled bool) error {
	return s.repo.SetMaintenance(ctx, serverID, enabled, s.now())
}

func (s *Service) MarkStale(ctx context.Context) error {
	now := s.now()
	return s.repo.MarkStale(ctx, now.Add(-time.Minute), now.Add(-5*time.Minute), now)
}
