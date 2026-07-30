package controlplane

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/anrted/opendeploy/pkg/controlcapabilities"
	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
	"github.com/google/uuid"
)

var (
	ErrAgentOffline     = errors.New("agent is offline")
	ErrConnectionClosed = errors.New("agent connection closed")
	ErrCapabilityAbsent = errors.New("agent capability is unavailable")
)

const ProtocolVersion uint32 = 1

type pendingResult struct {
	result *agentv1.CommandResult
	err    error
}

type session struct {
	id           string
	serverID     string
	apiVersion   string
	agentVersion string
	capabilities map[string]struct{}
	send         chan *agentv1.CoreMessage
	done         chan struct{}
	pending      map[string]chan pendingResult
	streams      map[string]chan *agentv1.StreamChunk
	mu           sync.Mutex
}

// Manager owns all live Agent streams. It is safe for concurrent use by HTTP
// handlers and background task dispatchers.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*session
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*session)}
}

func (m *Manager) register(hello *agentv1.AgentHello) (*session, error) {
	if hello.GetProtocolVersion() != ProtocolVersion {
		return nil, fmt.Errorf("unsupported control-plane protocol %d", hello.GetProtocolVersion())
	}
	capabilities := make(map[string]struct{}, len(hello.GetCapabilities()))
	for _, capability := range hello.GetCapabilities() {
		capabilities[capability] = struct{}{}
	}
	s := &session{
		id: uuid.NewString(), serverID: hello.GetServerId(),
		apiVersion: hello.GetApiVersion(), agentVersion: hello.GetAgentVersion(),
		capabilities: capabilities,
		send:         make(chan *agentv1.CoreMessage, 64), done: make(chan struct{}),
		pending: make(map[string]chan pendingResult),
		streams: make(map[string]chan *agentv1.StreamChunk),
	}
	m.mu.Lock()
	if previous := m.sessions[s.serverID]; previous != nil {
		previous.close(ErrConnectionClosed)
	}
	m.sessions[s.serverID] = s
	m.mu.Unlock()
	return s, nil
}

func (m *Manager) unregister(s *session) {
	m.mu.Lock()
	if m.sessions[s.serverID] == s {
		delete(m.sessions, s.serverID)
	}
	m.mu.Unlock()
	s.close(ErrConnectionClosed)
}

func (s *session) close(err error) {
	s.mu.Lock()
	select {
	case <-s.done:
		s.mu.Unlock()
		return
	default:
		close(s.done)
	}
	for id, waiter := range s.pending {
		delete(s.pending, id)
		select {
		case waiter <- pendingResult{err: err}:
		default:
		}
	}
	for id := range s.streams {
		delete(s.streams, id)
	}
	s.mu.Unlock()
}

func (m *Manager) Subscribe(ctx context.Context, serverID, kind string, payload []byte) (<-chan []byte, error) {
	m.mu.RLock()
	s := m.sessions[serverID]
	m.mu.RUnlock()
	if s == nil {
		return nil, ErrAgentOffline
	}
	if err := m.requireCommandCapability(s, kind); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	chunks := make(chan *agentv1.StreamChunk, 64)
	output := make(chan []byte, 64)
	s.mu.Lock()
	select {
	case <-s.done:
		s.mu.Unlock()
		return nil, ErrConnectionClosed
	default:
	}
	s.streams[id] = chunks
	s.mu.Unlock()
	removeStream := func() {
		s.mu.Lock()
		delete(s.streams, id)
		s.mu.Unlock()
	}
	message := &agentv1.CoreMessage{Body: &agentv1.CoreMessage_Subscribe{Subscribe: &agentv1.StreamSubscription{
		Id: id, Kind: kind, Payload: payload,
	}}}
	select {
	case s.send <- message:
	case <-s.done:
		removeStream()
		return nil, ErrConnectionClosed
	case <-ctx.Done():
		removeStream()
		return nil, ctx.Err()
	}
	go func() {
		defer close(output)
		defer func() {
			removeStream()
			select {
			case s.send <- &agentv1.CoreMessage{Body: &agentv1.CoreMessage_Cancel{Cancel: &agentv1.StreamCancel{Id: id}}}:
			default:
			}
		}()
		for {
			select {
			case chunk, ok := <-chunks:
				if !ok || chunk.GetEof() {
					return
				}
				select {
				case output <- chunk.GetData():
				case <-ctx.Done():
					return
				case <-s.done:
					return
				}
			case <-ctx.Done():
				return
			case <-s.done:
				return
			}
		}
	}()
	return output, nil
}

func (s *session) resolveChunk(chunk *agentv1.StreamChunk) {
	s.mu.Lock()
	stream := s.streams[chunk.GetStreamId()]
	s.mu.Unlock()
	if stream != nil {
		select {
		case stream <- chunk:
		case <-s.done:
		}
	}
}

func (m *Manager) IsOnline(serverID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[serverID] != nil
}

func (m *Manager) HasCapability(serverID, capability string) bool {
	m.mu.RLock()
	s := m.sessions[serverID]
	m.mu.RUnlock()
	if s == nil {
		return false
	}
	_, ok := s.capabilities[capability]
	return ok
}

func (m *Manager) requireCommandCapability(s *session, kind string) error {
	required, known := controlcapabilities.RequiredForCommand(kind)
	if !known {
		return fmt.Errorf("%w: command %q is not registered", ErrCapabilityAbsent, kind)
	}
	if _, ok := s.capabilities[required]; !ok {
		return fmt.Errorf("%w: %s", ErrCapabilityAbsent, required)
	}
	return nil
}

func (m *Manager) Capabilities(serverID string) []string {
	m.mu.RLock()
	s := m.sessions[serverID]
	m.mu.RUnlock()
	if s == nil {
		return nil
	}
	result := make([]string, 0, len(s.capabilities))
	for capability := range s.capabilities {
		result = append(result, capability)
	}
	sort.Strings(result)
	return result
}

func (m *Manager) Dispatch(ctx context.Context, serverID, kind string, payload []byte) ([]byte, error) {
	m.mu.RLock()
	s := m.sessions[serverID]
	m.mu.RUnlock()
	if s == nil {
		return nil, ErrAgentOffline
	}
	if err := m.requireCommandCapability(s, kind); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	waiter := make(chan pendingResult, 1)
	s.mu.Lock()
	s.pending[id] = waiter
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()
	deadline := time.Now().Add(30 * time.Second)
	if value, ok := ctx.Deadline(); ok {
		deadline = value
	}
	message := &agentv1.CoreMessage{Body: &agentv1.CoreMessage_Command{Command: &agentv1.AgentCommand{
		Id: id, Kind: kind, Payload: payload, DeadlineUnixMs: deadline.UnixMilli(),
	}}}
	select {
	case s.send <- message:
	case <-s.done:
		return nil, ErrConnectionClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case outcome := <-waiter:
		if outcome.err != nil {
			return nil, outcome.err
		}
		if !outcome.result.GetSuccess() {
			return nil, errors.New(outcome.result.GetError())
		}
		return outcome.result.GetPayload(), nil
	case <-s.done:
		return nil, ErrConnectionClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *session) resolve(result *agentv1.CommandResult) {
	s.mu.Lock()
	waiter := s.pending[result.GetId()]
	s.mu.Unlock()
	if waiter != nil {
		select {
		case waiter <- pendingResult{result: result}:
		default:
			// Duplicate/late results must not stall the shared receive loop.
		}
	}
}
