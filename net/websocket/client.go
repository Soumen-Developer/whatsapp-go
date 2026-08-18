// Package websocket implements the WhatsApp WebSocket connection with custom framing.
// Copyright (c) 2024 Soumen-Developer. All rights reserved.
// Licensed under MPL-2.0 (see LICENSE).
package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Soumen-Developer/whatsapp-go/binary"
	"github.com/gorilla/websocket"
)

const (
	// WhatsApp WebSocket endpoint
	DefaultEndpoint = "wss://web.whatsapp.com/ws"

	// WebSocket protocol version
	ProtocolVersion = "1"

	// Default timeouts
	DefaultConnectTimeout  = 30 * time.Second
	DefaultWriteTimeout    = 10 * time.Second
	DefaultReadTimeout     = 60 * time.Second
	DefaultPingInterval    = 30 * time.Second
	DefaultPongTimeout     = 10 * time.Second
	DefaultReconnectDelay  = 5 * time.Second
	MaxReconnectDelay      = 5 * time.Minute
	MaxReconnectAttempts   = 10

	// Frame types
	FrameTypeBinary = 0x00
	FrameTypePing   = 0x01
	FrameTypePong   = 0x02
	FrameTypeClose  = 0x03
)

// Client represents a WhatsApp WebSocket connection.
type Client struct {
	conn         *websocket.Conn
	endpoint     string
	headers      http.Header
	dialer       *websocket.Dialer
	
	// State
	mu           sync.RWMutex
	connected    bool
	reconnecting bool
	closed       bool
	
	// Channels
	sendCh       chan []byte
	recvCh       chan *Frame
	errCh        chan error
	closeCh      chan struct{}
	
	// Callbacks
	onConnect    func()
	onDisconnect func(error)
	onMessage    func(*Frame)
	onError      func(error)
	
	// Reconnection
	reconnectAttempts    int
	reconnectDelay       time.Duration
	maxReconnectAttempts int
	maxReconnectDelay    time.Duration
	
	// Context
	ctx    context.Context
	cancel context.CancelFunc
	
	// Ping/pong
	pingInterval time.Duration
	pongTimeout  time.Duration
	lastPong     time.Time
}

// Frame represents a WhatsApp WebSocket frame.
type Frame struct {
	Type      byte
	Payload   []byte
	Timestamp time.Time
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithEndpoint sets a custom WebSocket endpoint.
func WithEndpoint(endpoint string) ClientOption {
	return func(c *Client) {
		c.endpoint = endpoint
	}
}

// WithHeaders sets custom HTTP headers for the WebSocket handshake.
func WithHeaders(headers http.Header) ClientOption {
	return func(c *Client) {
		for k, v := range headers {
			c.headers[k] = v
		}
	}
}

// WithConnectTimeout sets the connection timeout.
func WithConnectTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.dialer.HandshakeTimeout = timeout
	}
}

// WithCallbacks sets event callbacks.
func WithCallbacks(onConnect func(), onDisconnect func(error), onMessage func(*Frame), onError func(error)) ClientOption {
	return func(c *Client) {
		c.onConnect = onConnect
		c.onDisconnect = onDisconnect
		c.onMessage = onMessage
		c.onError = onError
	}
}

// WithReconnectConfig configures reconnection behavior.
func WithReconnectConfig(maxAttempts int, initialDelay, maxDelay time.Duration) ClientOption {
	return func(c *Client) {
		c.reconnectDelay = initialDelay
		if maxDelay > 0 {
			c.maxReconnectDelay = maxDelay
		}
		c.maxReconnectAttempts = maxAttempts
	}
}

// NewClient creates a new WhatsApp WebSocket client.
func NewClient(opts ...ClientOption) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	
	c := &Client{
		endpoint:       DefaultEndpoint,
		headers:        make(http.Header),
		dialer:         websocket.DefaultDialer,
		sendCh:         make(chan []byte, 256),
		recvCh:         make(chan *Frame, 256),
		errCh:          make(chan error, 16),
		closeCh:        make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
		reconnectDelay: DefaultReconnectDelay,
		pingInterval:   DefaultPingInterval,
		pongTimeout:    DefaultPongTimeout,
		lastPong:       time.Now(),
	}
	
	// Default headers for WhatsApp
	c.headers.Set("User-Agent", "WhatsApp/2.2412.54 Chrome/120.0.0.0 Safari/537.36")
	c.headers.Set("Origin", "https://web.whatsapp.com")
	c.headers.Set("Accept-Encoding", "gzip, deflate, br")
	c.headers.Set("Accept-Language", "en-US,en;q=0.9")
	c.headers.Set("Cache-Control", "no-cache")
	c.headers.Set("Pragma", "no-cache")
	// Note: Sec-WebSocket-Extensions and Sec-WebSocket-Version are handled by gorilla/websocket
	
	// TLS config - WhatsApp uses valid certs, but allow insecure for testing
	c.dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: false,
	}
	
	// Apply options
	for _, opt := range opts {
		opt(c)
	}
	
	return c
}

// Connect establishes the WebSocket connection and starts read/write loops.
func (c *Client) Connect() error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return errors.New("already connected")
	}
	if c.closed {
		c.mu.Unlock()
		return errors.New("client is closed")
	}
	c.mu.Unlock()
	
	// Build URL with query parameters
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint: %w", err)
	}
	
	// Add WhatsApp-specific query params
	q := u.Query()
	q.Set("version", ProtocolVersion)
	q.Set("browser", "Chrome")
	q.Set("platform", "Web")
	u.RawQuery = q.Encode()
	
	// Dial
	conn, resp, err := c.dialer.DialContext(c.ctx, u.String(), c.headers)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket handshake failed (status %d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("websocket dial failed: %w", err)
	}
	
	c.conn = conn
	c.conn.SetReadLimit(16 * 1024 * 1024) // 16MB max message
	
	c.mu.Lock()
	c.connected = true
	c.reconnectAttempts = 0
	c.mu.Unlock()
	
	// Start goroutines
	go c.readLoop()
	go c.writeLoop()
	go c.pingLoop()
	
	// Trigger connect callback
	if c.onConnect != nil {
		c.onConnect()
	}
	
	return nil
}

// Disconnect closes the connection gracefully.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.cancel()
	c.mu.Unlock()
	
	close(c.closeCh)
	
	if c.conn != nil {
		// Send close frame
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.conn.Close()
	}
	
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	
	if c.onDisconnect != nil {
		c.onDisconnect(nil)
	}
	
	return nil
}

// IsConnected returns true if the client is connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Send sends a binary frame.
func (c *Client) Send(payload []byte) error {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()
	
	if !connected {
		return errors.New("not connected")
	}
	
	select {
	case c.sendCh <- payload:
		return nil
	case <-c.closeCh:
		return errors.New("client closed")
	case <-time.After(DefaultWriteTimeout):
		return errors.New("send timeout")
	}
}

// SendFrame sends a frame with specific type.
func (c *Client) SendFrame(frameType byte, payload []byte) error {
	frame := encodeFrame(frameType, payload)
	return c.Send(frame)
}

// Recv returns the next received frame (blocking).
func (c *Client) Recv() (*Frame, error) {
	select {
	case frame := <-c.recvCh:
		return frame, nil
	case err := <-c.errCh:
		return nil, err
	case <-c.closeCh:
		return nil, errors.New("client closed")
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

// Errors returns the error channel.
func (c *Client) Errors() <-chan error {
	return c.errCh
}

// readLoop reads frames from the WebSocket connection.
func (c *Client) readLoop() {
	defer func() {
		c.handleDisconnect(nil)
	}()
	
	for {
		select {
		case <-c.closeCh:
			return
		case <-c.ctx.Done():
			return
		default:
		}
		
		// Set read deadline
		c.conn.SetReadDeadline(time.Now().Add(DefaultReadTimeout))
		
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.errCh <- fmt.Errorf("websocket read error: %w", err)
			}
			return
		}
		
		// Decode frame
		frame, err := decodeFrame(message)
		if err != nil {
			c.errCh <- fmt.Errorf("frame decode error: %w", err)
			continue
		}
		
		// Handle special frames
		switch frame.Type {
		case FrameTypePong:
			c.mu.Lock()
			c.lastPong = time.Now()
			c.mu.Unlock()
			continue
		case FrameTypeClose:
			c.handleDisconnect(errors.New("received close frame"))
			return
		}
		
		// Deliver frame
		select {
		case c.recvCh <- frame:
		case <-c.closeCh:
			return
		default:
			// Channel full, drop frame
			c.errCh <- errors.New("recv channel full, frame dropped")
		}
		
		// Trigger message callback
		if c.onMessage != nil {
			c.onMessage(frame)
		}
	}
}

// writeLoop writes frames to the WebSocket connection.
func (c *Client) writeLoop() {
	ticker := time.NewTicker(DefaultWriteTimeout)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.closeCh:
			return
		case <-c.ctx.Done():
			return
		case payload := <-c.sendCh:
			c.conn.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout))
			if err := c.conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
				c.errCh <- fmt.Errorf("websocket write error: %w", err)
				return
			}
		case <-ticker.C:
			// Flush any pending writes
		}
	}
}

// pingLoop sends periodic pings to keep connection alive.
func (c *Client) pingLoop() {
	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.closeCh:
			return
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			connected := c.connected
			lastPong := c.lastPong
			c.mu.RUnlock()
			
			if !connected {
				return
			}
			
			// Check pong timeout
			if time.Since(lastPong) > c.pongTimeout+c.pingInterval {
				c.errCh <- errors.New("pong timeout, connection stale")
				c.handleDisconnect(errors.New("pong timeout"))
				return
			}
			
			// Send ping
			if err := c.SendFrame(FrameTypePing, []byte{}); err != nil {
				c.errCh <- fmt.Errorf("ping send failed: %w", err)
				return
			}
		}
	}
}

// handleDisconnect handles connection loss and triggers reconnection.
func (c *Client) handleDisconnect(err error) {
	c.mu.Lock()
	wasConnected := c.connected
	c.connected = false
	closed := c.closed
	c.mu.Unlock()
	
	if wasConnected && !closed {
		if c.onDisconnect != nil {
			c.onDisconnect(err)
		}
		
		// Attempt reconnection
		go c.attemptReconnect(err)
	}
}

// attemptReconnect tries to reconnect with exponential backoff.
func (c *Client) attemptReconnect(lastErr error) {
	c.mu.Lock()
	if c.closed || c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	maxAttempts := c.maxReconnectAttempts
	if maxAttempts <= 0 {
		maxAttempts = MaxReconnectAttempts
	}
	maxDelay := c.maxReconnectDelay
	if maxDelay <= 0 {
		maxDelay = MaxReconnectDelay
	}
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()

	for attempt := 0; attempt < maxAttempts; attempt++ {
		c.mu.RLock()
		closed := c.closed
		c.mu.RUnlock()

		if closed {
			return
		}

		delay := c.reconnectDelay
		for i := 0; i < attempt; i++ {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
				break
			}
		}

		// Add jitter
		jitter := time.Duration(float64(delay) * 0.1 * (2*float64(time.Now().UnixNano()%1000)/1000 - 1))
		delay += jitter

		select {
		case <-c.closeCh:
			return
		case <-c.ctx.Done():
			return
		case <-time.After(delay):
		}

		// Try to reconnect
		err := c.Connect()
		if err == nil {
			// Reconnected successfully
			if c.onError != nil {
				c.onError(fmt.Errorf("reconnected after disconnect: %w", lastErr))
			}
			return
		}

		c.mu.Lock()
		c.reconnectAttempts = attempt + 1
		c.mu.Unlock()

		if c.onError != nil {
			c.onError(fmt.Errorf("reconnect attempt %d failed: %w", attempt+1, err))
		}
	}

	// Max attempts reached
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	if c.onError != nil {
		c.onError(fmt.Errorf("max reconnect attempts reached, giving up: %w", lastErr))
	}

	c.Disconnect()
}

// encodeFrame encodes a frame to binary format.
// WhatsApp frame format: [type:1][length:4][payload...]
func encodeFrame(frameType byte, payload []byte) []byte {
	length := uint32(len(payload))
	frame := make([]byte, 5+len(payload))
	frame[0] = frameType
	// Length as big-endian uint32
	frame[1] = byte(length >> 24)
	frame[2] = byte(length >> 16)
	frame[3] = byte(length >> 8)
	frame[4] = byte(length)
	copy(frame[5:], payload)
	return frame
}

// decodeFrame decodes a frame from binary format.
func decodeFrame(data []byte) (*Frame, error) {
	if len(data) < 5 {
		return nil, errors.New("frame too short")
	}

	frameType := data[0]
	length := uint32(data[1])<<24 | uint32(data[2])<<16 | uint32(data[3])<<8 | uint32(data[4])

	if uint32(len(data)) < 5+length {
		return nil, errors.New("frame payload truncated")
	}

	payload := data[5 : 5+length]

	return &Frame{
		Type:      frameType,
		Payload:   payload,
		Timestamp: time.Now(),
	}, nil
}

// ParseBinaryNode parses a binary node from frame payload.
func ParseBinaryNode(payload []byte) (*binary.Node, error) {
	return binary.Decode(payload)
}