// Package session implements WhatsApp session management and QR pairing.
// Copyright (c) 2024 Soumen-Developer. All rights reserved.
// Licensed under MPL-2.0 (see LICENSE).
package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Soumen-Developer/whatsapp-go/crypto"
	"github.com/Soumen-Developer/whatsapp-go/net/websocket"
	whatsapp "github.com/Soumen-Developer/whatsapp-go/types"
)

const (
	// Session file name
	SessionFile = "sessions.json"

	// Pairing constants
	PairingCodeLength = 8
	PairingTimeout    = 60 * time.Second
	QRCodeTimeout     = 45 * time.Second
)

// Session represents a WhatsApp session.
type Session struct {
	ID             string             `json:"id"`
	JID            *whatsapp.JID      `json:"jid"`
	NoiseStaticKey *[32]byte          `json:"static_key"`
	NoisePeerKey   *[32]byte          `json:"peer_key"`
	EncryptionKey  *[32]byte          `json:"encryption_key"`
	MACKey         *[32]byte          `json:"mac_key"`
	DeviceID       string             `json:"device_id"`
	Platform       string             `json:"platform"`
	BusinessName   string             `json:"business_name"`
	RegisteredAt   time.Time          `json:"registered_at"`
	LastActive     time.Time          `json:"last_active"`
	IsActive       bool               `json:"is_active"`
	Props          map[string]string  `json:"props"`
}

// PairingInfo holds QR pairing information.
type PairingInfo struct {
	Code        string    `json:"code"`
	Ref         string    `json:"ref"`
	ExpiresAt   time.Time `json:"expires_at"`
	QRCodePNG   []byte    `json:"qr_code_png,omitempty"`
	QRCodeData  string    `json:"qr_code_data"` // Base64 encoded PNG
}

// Store manages session persistence (file-based JSON).
type Store struct {
	path      string
	mu        sync.RWMutex
	sessions  map[string]*Session
}

// NewStore creates a new session store.
func NewStore(dbPath string) (*Store, error) {
	if dbPath == "" {
		dbPath = SessionFile
	}
	
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}
	
	s := &Store{
		path:     dbPath,
		sessions: make(map[string]*Session),
	}
	
	// Load sessions from file
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load sessions: %w", err)
	}
	
	return s, nil
}

// load loads sessions from JSON file.
func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	
	if len(data) == 0 {
		return nil
	}
	
	var sessions []*Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return err
	}
	
	for _, sess := range sessions {
		if sess.IsActive {
			s.sessions[sess.ID] = sess
		}
	}
	
	return nil
}

// save saves sessions to JSON file.
// Must be called with s.mu already locked.
func (s *Store) save() error {
	sessions := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0600)
}

// Save persists a session to the store.
func (s *Store) Save(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if session.Props == nil {
		session.Props = make(map[string]string)
	}
	
	s.sessions[session.ID] = session
	return s.save()
}

// Get retrieves a session by ID.
func (s *Store) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if sess, ok := s.sessions[id]; ok {
		return sess, nil
	}
	return nil, errors.New("session not found")
}

// GetByJID retrieves a session by JID.
func (s *Store) GetByJID(jid *whatsapp.JID) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, sess := range s.sessions {
		if sess.JID.User == jid.User && sess.JID.Server == jid.Server {
			return sess, nil
		}
	}
	return nil, errors.New("session not found")
}

// List returns all active sessions.
func (s *Store) List() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	sessions := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	return sessions
}

// Delete marks a session as inactive.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if sess, ok := s.sessions[id]; ok {
		sess.IsActive = false
		delete(s.sessions, id)
		return s.save()
	}
	return errors.New("session not found")
}

// UpdateActivity updates the last active time.
func (s *Store) UpdateActivity(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if sess, ok := s.sessions[id]; ok {
		sess.LastActive = time.Now()
		return s.save()
	}
	return errors.New("session not found")
}

// Close closes the store (no-op for file-based).
func (s *Store) Close() error {
	return nil
}

// PairingClient handles QR code pairing flow.
type PairingClient struct {
	wsClient    *websocket.Client
	store       *Store
	session     *Session
	pairingInfo *PairingInfo
	
	mu           sync.Mutex
	paired       bool
	pairingErr   error
	pairingDone  chan struct{}
}

// NewPairingClient creates a new pairing client.
func NewPairingClient(store *Store, wsClient *websocket.Client) *PairingClient {
	return &PairingClient{
		wsClient:   wsClient,
		store:      store,
		pairingDone: make(chan struct{}),
	}
}

// StartPairing initiates the QR pairing flow.
func (p *PairingClient) StartPairing(ctx context.Context, phoneNumber string) (*PairingInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.paired {
		return nil, errors.New("already paired")
	}
	
	// Generate pairing code (8 digits)
	codeBytes, err := crypto.RandomBytes(PairingCodeLength / 2)
	if err != nil {
		return nil, err
	}
	code := fmt.Sprintf("%08x", codeBytes)[:PairingCodeLength]
	
	// Generate ref
	refBytes, err := crypto.RandomBytes(16)
	if err != nil {
		return nil, err
	}
	ref := base64.URLEncoding.EncodeToString(refBytes)
	
	// Create pairing info
	p.pairingInfo = &PairingInfo{
		Code:      code,
		Ref:       ref,
		ExpiresAt: time.Now().Add(PairingTimeout),
	}
	
	// Generate QR code data (WhatsApp format: ref,code,version)
	qrData := fmt.Sprintf("%s,%s,1", ref, code)
	p.pairingInfo.QRCodeData = base64.StdEncoding.EncodeToString([]byte(qrData))
	
	// Generate QR code PNG (placeholder - would use a QR library)
	p.pairingInfo.QRCodePNG = []byte(qrData) // Simplified
	
	// Set up WebSocket callbacks
	p.wsClient = websocket.NewClient(
		websocket.WithCallbacks(
			p.onConnect,
			p.onDisconnect,
			p.onMessage,
			p.onError,
		),
	)
	
	// Connect
	if err := p.wsClient.Connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	
	// Send pairing request
	pairMsg := map[string]interface{}{
		"type": "pair",
		"ref":  ref,
		"code": code,
	}
	data, _ := json.Marshal(pairMsg)
	
	if err := p.wsClient.SendFrame(websocket.FrameTypeBinary, data); err != nil {
		return nil, fmt.Errorf("send pairing request: %w", err)
	}
	
	// Wait for pairing completion or timeout
	select {
	case <-ctx.Done():
		p.wsClient.Disconnect()
		return nil, ctx.Err()
	case <-time.After(QRCodeTimeout):
		p.wsClient.Disconnect()
		return nil, errors.New("pairing timeout")
	case <-p.pairingDone:
		if p.pairingErr != nil {
			return nil, p.pairingErr
		}
		return p.pairingInfo, nil
	}
}

// onConnect handles WebSocket connection.
func (p *PairingClient) onConnect() {
	// Connection established, pairing request already sent
}

// onDisconnect handles WebSocket disconnection.
func (p *PairingClient) onDisconnect(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.paired && p.pairingErr == nil {
		p.pairingErr = fmt.Errorf("disconnected during pairing: %w", err)
		close(p.pairingDone)
	}
}

// onMessage handles incoming messages.
func (p *PairingClient) onMessage(frame *websocket.Frame) {
	if frame.Type != websocket.FrameTypeBinary {
		return
	}
	
	var msg map[string]interface{}
	if err := json.Unmarshal(frame.Payload, &msg); err != nil {
		return
	}
	
	msgType, _ := msg["type"].(string)
	
	switch msgType {
	case "pair_success":
		p.handlePairSuccess(msg)
	case "pair_failure":
		p.handlePairFailure(msg)
	case "challenge":
		p.handleChallenge(msg)
	}
}

// handlePairSuccess handles successful pairing.
func (p *PairingClient) handlePairSuccess(msg map[string]interface{}) {
	// Extract session data
	jidStr, _ := msg["jid"].(string)
	staticKeyB64, _ := msg["static_key"].(string)
	peerKeyB64, _ := msg["peer_key"].(string)
	encKeyB64, _ := msg["encryption_key"].(string)
	macKeyB64, _ := msg["mac_key"].(string)
	deviceID, _ := msg["device_id"].(string)
	platform, _ := msg["platform"].(string)
	businessName, _ := msg["business_name"].(string)
	
	// Parse JID
	jid, err := whatsapp.ParseJID(jidStr)
	if err != nil {
		p.setError(fmt.Errorf("invalid JID: %w", err))
		return
	}
	
	// Decode keys
	staticKey, _ := base64.StdEncoding.DecodeString(staticKeyB64)
	peerKey, _ := base64.StdEncoding.DecodeString(peerKeyB64)
	encKey, _ := base64.StdEncoding.DecodeString(encKeyB64)
	macKey, _ := base64.StdEncoding.DecodeString(macKeyB64)
	
	// Create session
	p.session = &Session{
		ID:             generateSessionID(),
		JID:            jid,
		NoiseStaticKey: (*[32]byte)(staticKey[:32]),
		NoisePeerKey:   (*[32]byte)(peerKey[:32]),
		EncryptionKey:  (*[32]byte)(encKey[:32]),
		MACKey:         (*[32]byte)(macKey[:32]),
		DeviceID:       deviceID,
		Platform:       platform,
		BusinessName:   businessName,
		RegisteredAt:   time.Now(),
		LastActive:     time.Now(),
		IsActive:       true,
		Props:          make(map[string]string),
	}
	
	// Save session
	if err := p.store.Save(p.session); err != nil {
		p.setError(fmt.Errorf("save session: %w", err))
		return
	}
	
	p.mu.Lock()
	p.paired = true
	p.mu.Unlock()
	
	close(p.pairingDone)
}

// handlePairFailure handles pairing failure.
func (p *PairingClient) handlePairFailure(msg map[string]interface{}) {
	reason, _ := msg["reason"].(string)
	p.setError(fmt.Errorf("pairing failed: %s", reason))
}

// handleChallenge handles pairing challenge (for business accounts).
func (p *PairingClient) handleChallenge(msg map[string]interface{}) {
	// Send challenge response
	challenge, _ := msg["challenge"].(string)
	response := map[string]interface{}{
		"type":       "challenge_response",
		"challenge":  challenge,
		"response":   "accepted",
	}
	data, _ := json.Marshal(response)
	p.wsClient.SendFrame(websocket.FrameTypeBinary, data)
}

// onError handles WebSocket errors.
func (p *PairingClient) onError(err error) {
	if !p.paired {
		p.setError(err)
	}
}

func (p *PairingClient) setError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pairingErr = err
	select {
	case <-p.pairingDone:
	default:
		close(p.pairingDone)
	}
}

// GetSession returns the paired session.
func (p *PairingClient) GetSession() *Session {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.session
}

// IsPaired returns true if pairing completed successfully.
func (p *PairingClient) IsPaired() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paired
}

// GenerateQRCode generates a QR code for pairing.
// This is a placeholder - in production use a QR library like github.com/skip2/go-qrcode
func GenerateQRCode(data string) ([]byte, error) {
	// Placeholder: return base64 encoded data
	// Real implementation would use: qrcode.Encode(data, qrcode.Medium, 256)
	return []byte(data), nil
}

// GeneratePairingQR generates a QR code image for the pairing info.
func (p *PairingInfo) GenerateQRCode() ([]byte, error) {
	return GenerateQRCode(p.QRCodeData)
}

// Helper functions

func generateSessionID() string {
	b, err := crypto.RandomBytes(16)
	if err != nil {
		// Fallback to time-based ID
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return base64.URLEncoding.EncodeToString(b)
}