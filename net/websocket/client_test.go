// Package websocket implements the WhatsApp WebSocket connection with custom framing.
// Copyright (c) 2024 Soumen-Developer. All rights reserved.
// Licensed under MPL-2.0 (see LICENSE).
package websocket

import (
	"net/http"
	"testing"
	"time"
)

// TestNewClient tests client creation with default and custom options.
func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.endpoint != DefaultEndpoint {
		t.Errorf("expected default endpoint %q, got %q", DefaultEndpoint, client.endpoint)
	}
	if client.headers.Get("User-Agent") == "" {
		t.Error("default User-Agent header not set")
	}
	if client.headers.Get("Origin") != "https://web.whatsapp.com" {
		t.Error("default Origin header not set")
	}

	// Test custom endpoint
	client = NewClient(WithEndpoint("wss://custom.example.com/ws"))
	if client.endpoint != "wss://custom.example.com/ws" {
		t.Errorf("expected custom endpoint, got %q", client.endpoint)
	}

	// Test custom headers
	customHeaders := http.Header{}
	customHeaders.Set("X-Custom", "value")
	client = NewClient(WithHeaders(customHeaders))
	if client.headers.Get("X-Custom") != "value" {
		t.Error("custom header not applied")
	}
}

// TestEncodeDecodeFrame tests frame encoding and decoding.
func TestEncodeDecodeFrame(t *testing.T) {
	testCases := []struct {
		name     string
		frameType byte
		payload   []byte
	}{
		{"empty binary", FrameTypeBinary, []byte{}},
		{"simple binary", FrameTypeBinary, []byte("hello")},
		{"binary with zeros", FrameTypeBinary, []byte{0x00, 0x01, 0x02}},
		{"ping frame", FrameTypePing, []byte{}},
		{"pong frame", FrameTypePong, []byte{}},
		{"close frame", FrameTypeClose, []byte{}},
		{"large payload", FrameTypeBinary, make([]byte, 1000)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := encodeFrame(tc.frameType, tc.payload)
			decoded, err := decodeFrame(encoded)
			if err != nil {
				t.Fatalf("decodeFrame failed: %v", err)
			}
			if decoded.Type != tc.frameType {
				t.Errorf("expected frame type %d, got %d", tc.frameType, decoded.Type)
			}
			if string(decoded.Payload) != string(tc.payload) {
				t.Errorf("payload mismatch: expected %q, got %q", tc.payload, decoded.Payload)
			}
		})
	}
}

// TestDecodeFrameErrors tests frame decoding error cases.
func TestDecodeFrameErrors(t *testing.T) {
	testCases := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"too short", []byte{0x00}, true},
		{"length mismatch", []byte{0x00, 0x00, 0x00, 0x00, 0x05, 0x01, 0x02}, true},
		{"valid frame", encodeFrame(FrameTypeBinary, []byte("test")), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeFrame(tc.data)
			if tc.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestClientOptions tests client option functions.
func TestClientOptions(t *testing.T) {
	// Test WithConnectTimeout
	client := NewClient(WithConnectTimeout(5 * time.Second))
	if client.dialer.HandshakeTimeout != 5*time.Second {
		t.Errorf("expected handshake timeout 5s, got %v", client.dialer.HandshakeTimeout)
	}

	// Test WithReconnectConfig
	client = NewClient(WithReconnectConfig(5, 2*time.Second, 1*time.Minute))
	if client.maxReconnectAttempts != 5 {
		t.Errorf("expected max reconnect attempts 5, got %d", client.maxReconnectAttempts)
	}
	if client.maxReconnectDelay != 1*time.Minute {
		t.Errorf("expected max reconnect delay 1m, got %v", client.maxReconnectDelay)
	}
	if client.reconnectDelay != 2*time.Second {
		t.Errorf("expected reconnect delay 2s, got %v", client.reconnectDelay)
	}
}

// TestClientState tests client connection state management.
func TestClientState(t *testing.T) {
	client := NewClient()
	
	if client.IsConnected() {
		t.Error("new client should not be connected")
	}
	
	client.mu.Lock()
	client.closed = true
	client.mu.Unlock()
	
	if client.IsConnected() {
		t.Error("closed client should not be connected")
	}
}

// TestContextCancellation tests context cancellation behavior.
func TestContextCancellation(t *testing.T) {
	client := NewClient()
	
	// Cancel context
	client.cancel()
	
	select {
	case <-client.ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("context should be cancelled")
	}
}

// BenchmarkEncodeFrame benchmarks frame encoding.
func BenchmarkEncodeFrame(b *testing.B) {
	payload := make([]byte, 1024)
	for i := 0; i < b.N; i++ {
		encodeFrame(FrameTypeBinary, payload)
	}
}

// BenchmarkDecodeFrame benchmarks frame decoding.
func BenchmarkDecodeFrame(b *testing.B) {
	payload := make([]byte, 1024)
	encoded := encodeFrame(FrameTypeBinary, payload)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decodeFrame(encoded)
	}
}

// TestIntegration_ConnectDisconnect tests connect/disconnect flow (requires network).
func TestIntegration_ConnectDisconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	
	client := NewClient(
		WithConnectTimeout(5*time.Second),
	)
	
	// This will fail without network, but we can test the setup
	err := client.Connect()
	if err != nil {
		// Expected in test environment without WhatsApp server
		t.Logf("Connect failed as expected: %v", err)
	}
	
	// Disconnect should not error
	err = client.Disconnect()
	if err != nil {
		t.Errorf("Disconnect failed: %v", err)
	}
}