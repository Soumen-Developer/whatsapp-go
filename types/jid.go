// Package whatsapp provides a WhatsApp Web Multidevice API client.
// Copyright (c) 2024 Soumen-Developer. All rights reserved.
// Licensed under MPL-2.0 (see LICENSE).
package whatsapp

import (
	"errors"
	"time"
)

// JID represents a WhatsApp ID (user, group, broadcast, etc.)
type JID struct {
	User   string
	Server string
	Device uint
	Agent  uint
}

func (j JID) String() string {
	if j.User == "" {
		return ""
	}
	if j.Server == "" {
		j.Server = DefaultUserServer
	}
	if j.Device > 0 {
		// Device is guaranteed to be single digit (0-9) by ParseJID
		deviceChar := byte('0' + j.Device)
		return j.User + ":" + string(deviceChar) + "@" + j.Server
	}
	return j.User + "@" + j.Server
}

func (j JID) IsEmpty() bool {
	return j.User == ""
}

func (j JID) ToNonAD() JID {
	return JID{User: j.User, Server: j.Server}
}

const (
	DefaultUserServer    = "s.whatsapp.net"
	GroupServer          = "g.us"
	BroadcastServer      = "broadcast"
	NewsletterServer     = "newsletter"
	HiddenUserServer     = "lid"
	BotServer            = "bot"
	ServerJID            = "s.whatsapp.net"
)

// ParseJID parses a JID string into a JID struct.
// Format: user@server or user:device@server
func ParseJID(s string) (*JID, error) {
	if s == "" {
		return nil, errors.New("empty JID")
	}
	
	// Find @ separator
	atIdx := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '@' {
			atIdx = i
			break
		}
	}
	if atIdx == -1 {
		return nil, errors.New("invalid JID format: missing @")
	}
	
	userPart := s[:atIdx]
	server := s[atIdx+1:]
	
	// Check for device in user part (user:device)
	device := uint(0)
	colonIdx := -1
	for i := len(userPart) - 1; i >= 0; i-- {
		if userPart[i] == ':' {
			colonIdx = i
			break
		}
	}
	
	user := userPart
	if colonIdx != -1 {
		user = userPart[:colonIdx]
		deviceStr := userPart[colonIdx+1:]
		// Parse device number
		var d uint64
		for _, c := range deviceStr {
			if c < '0' || c > '9' {
				return nil, errors.New("invalid device number")
			}
			d = d*10 + uint64(c-'0')
		}
		device = uint(d)
	}
	
	return &JID{
		User:   user,
		Server: server,
		Device: device,
		Agent:  0,
	}, nil
}

type MessageID string
type MessageServerID int64

// MessageSource contains sender/chat metadata
type MessageSource struct {
	Chat     JID
	Sender   JID
	IsFromMe bool
	IsGroup  bool
}

// MessageInfo contains full message metadata
type MessageInfo struct {
	MessageSource
	ID       MessageID
	ServerID MessageServerID
	Type     string
	PushName string
	Timestamp time.Time
}