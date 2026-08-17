// Package whatsapp provides a WhatsApp Web Multidevice API client.
// Copyright (c) 2024 Soumen Developer. All rights reserved.
// Licensed under MPL-2.0 (see LICENSE).
package whatsapp

import (
	"context"
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
		return j.User + ":" + string(rune('0'+j.Device)) + "@" + j.Server
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