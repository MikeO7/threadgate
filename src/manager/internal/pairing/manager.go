package pairing

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/hass"
)

var ErrNotFound = errors.New("pairing request expired or not found")

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusDenied   = "denied"
)

// Request represents a Home Assistant pairing session.
type Request struct {
	ID        string    `json:"pairing_id"`
	AppName   string    `json:"app_name"`
	AppURL    string    `json:"app_url"`
	HassToken string    `json:"hass_token"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"-"`
}

// Manager coordinates active in-memory pairing requests.
type Manager struct {
	mu       sync.Mutex
	requests map[string]*Request
}

// NewManager creates a manager for pairing requests.
func NewManager() *Manager {
	return &Manager{requests: make(map[string]*Request)}
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// AddRequest registers a new pairing request from Home Assistant.
func (m *Manager) AddRequest(appName, appURL, hassToken string) *Request {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, req := range m.requests {
		if now.After(req.ExpiresAt) {
			delete(m.requests, id)
		}
	}

	req := &Request{
		ID:        randomID(),
		AppName:   appName,
		AppURL:    appURL,
		HassToken: hassToken,
		Status:    StatusPending,
		ExpiresAt: now.Add(5 * time.Minute),
	}
	m.requests[req.ID] = req
	log.Printf("[Pairing] Initiated request %s from %s (%s)\n", req.ID, req.AppName, req.AppURL)
	return req
}

// GetRequest retrieves a pairing request.
func (m *Manager) GetRequest(id string) (*Request, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[id]
	if !ok || time.Now().After(req.ExpiresAt) {
		if ok {
			delete(m.requests, id)
		}
		return nil, false
	}
	return req, true
}

// GetActiveRequests returns all active pending pairing requests.
func (m *Manager) GetActiveRequests() []*Request {
	m.mu.Lock()
	defer m.mu.Unlock()

	var list []*Request
	now := time.Now()
	for id, req := range m.requests {
		if now.After(req.ExpiresAt) {
			delete(m.requests, id)
			continue
		}
		if req.Status == StatusPending {
			list = append(list, req)
		}
	}
	return list
}

// HasPending reports whether any pairing requests are awaiting approval.
func (m *Manager) HasPending() bool {
	return len(m.GetActiveRequests()) > 0
}

// ApproveAndSave approves a pairing request and persists Home Assistant credentials.
func (m *Manager) ApproveAndSave(id, stateDir string) (*Request, error) {
	req, ok := m.GetRequest(id)
	if !ok {
		return nil, ErrNotFound
	}
	if !m.Approve(id) {
		return nil, ErrNotFound
	}
	if err := hass.SaveConfig(stateDir, req.AppURL, req.HassToken); err != nil {
		return nil, err
	}
	return req, nil
}

// Approve sets a pairing request status to approved.
func (m *Manager) Approve(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[id]
	if !ok || time.Now().After(req.ExpiresAt) {
		return false
	}
	req.Status = StatusApproved
	log.Printf("[Pairing] Request %s approved by user\n", id)
	return true
}

// Deny sets a pairing request status to denied.
func (m *Manager) Deny(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[id]
	if !ok || time.Now().After(req.ExpiresAt) {
		return false
	}
	req.Status = StatusDenied
	log.Printf("[Pairing] Request %s denied by user\n", id)
	return true
}
