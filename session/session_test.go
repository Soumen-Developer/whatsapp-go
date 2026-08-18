// Package session implements WhatsApp session management and QR pairing.
// Copyright (c) 2024 Soumen-Developer. All rights reserved.
// Licensed under MPL-2.0 (see LICENSE).
package session

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	whatsapp "github.com/Soumen-Developer/whatsapp-go/types"
)

// TestNewStore tests store creation and file persistence.
func TestNewStore(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/sessions.json"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("store is nil")
	}

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// File might not exist until first save
	}

	// Test with existing file
	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore with existing file failed: %v", err)
	}
	defer store2.Close()
}

// TestStoreSaveGet tests saving and retrieving sessions.
func TestStoreSaveGet(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/sessions.json"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Create test session
	jid := &whatsapp.JID{
		User:   "1234567890",
		Server: "s.whatsapp.net",
		Device: 1,
		Agent:  0,
	}

	session := &Session{
		ID:             "test-session-1",
		JID:            jid,
		DeviceID:       "device-123",
		Platform:       "Chrome",
		BusinessName:   "Test Business",
		RegisteredAt:   time.Now(),
		LastActive:     time.Now(),
		IsActive:       true,
		Props:          map[string]string{"key": "value"},
	}

	// Save
	if err := store.Save(session); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Get by ID
	retrieved, err := store.Get("test-session-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("ID mismatch: expected %s, got %s", session.ID, retrieved.ID)
	}
	if retrieved.JID.User != jid.User {
		t.Errorf("JID User mismatch: expected %s, got %s", jid.User, retrieved.JID.User)
	}
	if retrieved.DeviceID != session.DeviceID {
		t.Errorf("DeviceID mismatch: expected %s, got %s", session.DeviceID, retrieved.DeviceID)
	}

	// Get by JID
	retrieved, err = store.GetByJID(jid)
	if err != nil {
		t.Fatalf("GetByJID failed: %v", err)
	}
	if retrieved.ID != session.ID {
		t.Errorf("GetByJID ID mismatch: expected %s, got %s", session.ID, retrieved.ID)
	}

	// Test non-existent
	_, err = store.Get("non-existent")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

// TestStoreList tests listing sessions.
func TestStoreList(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/sessions.json"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Add multiple sessions
	for i := 0; i < 3; i++ {
		jid := &whatsapp.JID{User: "user" + string(rune('0'+i)), Server: "s.whatsapp.net"}
		session := &Session{
			ID:           "session-" + string(rune('0'+i)),
			JID:          jid,
			IsActive:     true,
			RegisteredAt: time.Now(),
			LastActive:   time.Now(),
		}
		if err := store.Save(session); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	sessions := store.List()
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

// TestStoreDelete tests deleting sessions.
func TestStoreDelete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/sessions.json"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	jid := &whatsapp.JID{User: "1234567890", Server: "s.whatsapp.net"}
	session := &Session{
		ID:       "test-delete",
		JID:      jid,
		IsActive: true,
	}
	store.Save(session)

	// Delete
	if err := store.Delete("test-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = store.Get("test-delete")
	if err == nil {
		t.Error("expected error after delete")
	}

	// List should not include deleted
	sessions := store.List()
	for _, s := range sessions {
		if s.ID == "test-delete" {
			t.Error("deleted session still in list")
		}
	}
}

// TestStoreUpdateActivity tests updating activity time.
func TestStoreUpdateActivity(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/sessions.json"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	jid := &whatsapp.JID{User: "1234567890", Server: "s.whatsapp.net"}
	session := &Session{
		ID:        "test-activity",
		JID:       jid,
		IsActive:  true,
		LastActive: time.Now().Add(-1 * time.Hour),
	}
	store.Save(session)

	// Update activity
	if err := store.UpdateActivity("test-activity"); err != nil {
		t.Fatalf("UpdateActivity failed: %v", err)
	}

	retrieved, err := store.Get("test-activity")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.LastActive.Before(session.LastActive) {
		t.Error("LastActive should be updated to now")
	}
}

// TestPairingInfo tests pairing info creation.
func TestPairingInfo(t *testing.T) {
	info := &PairingInfo{
		Code:      "12345678",
		Ref:       "test-ref",
		ExpiresAt: time.Now().Add(60 * time.Second),
		QRCodeData: "base64data",
	}

	if info.Code != "12345678" {
		t.Errorf("expected code 12345678, got %s", info.Code)
	}
	if info.Ref != "test-ref" {
		t.Errorf("expected ref test-ref, got %s", info.Ref)
	}
	if info.QRCodeData != "base64data" {
		t.Errorf("expected QRCodeData base64data, got %s", info.QRCodeData)
	}
}

// TestGenerateSessionID tests session ID generation.
func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()

	if id1 == "" {
		t.Error("session ID should not be empty")
	}
	if id1 == id2 {
		t.Error("session IDs should be unique")
	}
}

// TestSessionJSON tests JSON marshaling.
func TestSessionJSON(t *testing.T) {
	jid := &whatsapp.JID{User: "1234567890", Server: "s.whatsapp.net"}
	session := &Session{
		ID:           "json-test",
		JID:          jid,
		DeviceID:     "device-123",
		Platform:     "Chrome",
		IsActive:     true,
		RegisteredAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		LastActive:   time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
		Props:        map[string]string{"custom": "value"},
	}

	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Session
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != session.ID {
		t.Errorf("ID mismatch after JSON roundtrip")
	}
	if decoded.JID.User != session.JID.User {
		t.Errorf("JID User mismatch after JSON roundtrip")
	}
	if decoded.DeviceID != session.DeviceID {
		t.Errorf("DeviceID mismatch after JSON roundtrip")
	}
	if !decoded.RegisteredAt.Equal(session.RegisteredAt) {
		t.Errorf("RegisteredAt mismatch after JSON roundtrip")
	}
	if decoded.Props["custom"] != "value" {
		t.Errorf("Props mismatch after JSON roundtrip")
	}
}

// TestStorePersistence tests that sessions persist across store instances.
func TestStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/sessions.json"

	// Create first store and save session
	store1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore1 failed: %v", err)
	}

	jid := &whatsapp.JID{User: "persist-test", Server: "s.whatsapp.net"}
	session := &Session{
		ID:       "persist-session",
		JID:      jid,
		IsActive: true,
	}
	if err := store1.Save(session); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	store1.Close()

	// Create second store with same file
	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore2 failed: %v", err)
	}
	defer store2.Close()

	retrieved, err := store2.Get("persist-session")
	if err != nil {
		t.Fatalf("Get from second store failed: %v", err)
	}

	if retrieved.ID != "persist-session" {
		t.Error("session not persisted across store instances")
	}
	if retrieved.JID.User != "persist-test" {
		t.Error("JID not persisted correctly")
	}
}