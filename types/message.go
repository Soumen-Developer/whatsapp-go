// Package whatsapp provides a WhatsApp Web Multidevice API client.
// Copyright (c) 2024 Soumen-Developer. All rights reserved.
// Licensed under MPL-2.0 (see LICENSE).
package whatsapp

import (
	"time"
)

// MessageType represents the type of a message
type MessageType string

const (
	MessageTypeText       MessageType = "text"
	MessageTypeImage      MessageType = "image"
	MessageTypeVideo      MessageType = "video"
	MessageTypeAudio      MessageType = "audio"
	MessageTypeDocument   MessageType = "document"
	MessageTypeSticker    MessageType = "sticker"
	MessageTypeLocation   MessageType = "location"
	MessageTypeContact    MessageType = "contact"
	MessageTypePoll       MessageType = "poll"
	MessageTypeReaction   MessageType = "reaction"
	MessageTypeEdit       MessageType = "edit"
	MessageTypeRevoke     MessageType = "revoke"
	MessageTypeEphemeral  MessageType = "ephemeral"
	MessageTypeViewOnce   MessageType = "view_once"
)

// MediaType represents the type of media
type MediaType string

const (
	MediaTypeImage     MediaType = "image"
	MediaTypeVideo     MediaType = "video"
	MediaTypeAudio     MediaType = "audio"
	MediaTypeDocument  MediaType = "document"
	MediaTypeSticker   MediaType = "sticker"
	MediaTypeThumbnail MediaType = "thumbnail"
)

// Message represents a WhatsApp message
type Message struct {
	ID        MessageID
	From      JID
	To        JID
	Timestamp time.Time
	Type      MessageType
	Text      string
	Image     *ImageMessage
	Video     *VideoMessage
	Audio     *AudioMessage
	Document  *DocumentMessage
	Sticker   *StickerMessage
	Location  *LocationMessage
	Contact   *ContactMessage
	Poll      *PollMessage
	Reaction  *ReactionMessage
	Edit      *EditedMessage
	Revoke    *RevokedMessage
	Ephemeral *EphemeralMessage
	ViewOnce  *ViewOnceMessage
	Quoted    *Message
	Source    MessageSource
}

// ImageMessage represents an image message
type ImageMessage struct {
	Caption       string
	MediaHandle   MediaHandle
	Mimetype      string
	FileLength    uint64
	Width         uint32
	Height        uint32
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileEncSHA256 []byte
	FileSHA256    []byte
	Thumbnail     []byte
}

// VideoMessage represents a video message
type VideoMessage struct {
	Caption       string
	MediaHandle   MediaHandle
	Mimetype      string
	FileLength    uint64
	Width         uint32
	Height        uint32
	Duration      uint32
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileEncSHA256 []byte
	FileSHA256    []byte
	Thumbnail     []byte
	GIFPlayback   bool
}

// AudioMessage represents an audio/voice message
type AudioMessage struct {
	MediaHandle   MediaHandle
	Mimetype      string
	FileLength    uint64
	Duration      uint32
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileEncSHA256 []byte
	FileSHA256    []byte
	PTT           bool // Push-to-talk (voice note)
	Waveform      []byte
}

// DocumentMessage represents a document message
type DocumentMessage struct {
	Caption       string
	MediaHandle   MediaHandle
	Mimetype      string
	FileLength    uint64
	FileName      string
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileEncSHA256 []byte
	FileSHA256    []byte
	Thumbnail     []byte
	PageCount     uint32
}

// StickerMessage represents a sticker message
type StickerMessage struct {
	MediaHandle   MediaHandle
	Mimetype      string
	FileLength    uint64
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileEncSHA256 []byte
	FileSHA256    []byte
	IsAnimated    bool
	IsAvatar      bool
	Thumbnail     []byte
}

// LocationMessage represents a location message
type LocationMessage struct {
	DegreesLatitude  float64
	DegreesLongitude float64
	Name             string
	Address          string
	URL              string
	Accuracy         float64
}

// ContactMessage represents a contact message
type ContactMessage struct {
	DisplayName string
	VCard       string
	Contacts    []Contact
}

// Contact represents a single contact
type Contact struct {
	DisplayName string
	GivenName   string
	FamilyName  string
	PhoneNumbers []string
	Emails      []string
	Addresses   []string
	URLs        []string
	Organization string
	Birthday    string
}

// PollMessage represents a poll message
type PollMessage struct {
	Name     string
	Options  []PollOption
	SelectableOptionsCount uint32
}

// PollOption represents a poll option
type PollOption struct {
	OptionName string
	VoteCount  uint32
}

// ReactionMessage represents a reaction message
type ReactionMessage struct {
	Key         string // Message ID being reacted to
	Reaction    string // Emoji
	Sender      JID
	Timestamp   time.Time
}

// EditedMessage represents an edited message
type EditedMessage struct {
	Key       string // Message ID being edited
	NewText   string
	Timestamp time.Time
}

// RevokedMessage represents a revoked (deleted) message
type RevokedMessage struct {
	Key       string // Message ID being revoked
	Timestamp time.Time
}

// EphemeralMessage represents an ephemeral (disappearing) message
type EphemeralMessage struct {
	Message   *Message
	Timer     uint32 // Seconds
	Timestamp time.Time
}

// ViewOnceMessage represents a view-once message
type ViewOnceMessage struct {
	Message   *Message
	MediaType MediaType
	Timestamp time.Time
}

// MediaHandle represents a handle to uploaded media
type MediaHandle struct {
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileEncSHA256 []byte
	FileSHA256    []byte
	FileLength    uint64
	Mimetype      string
	MediaType     MediaType
}

// GroupInfo represents group information
type GroupInfo struct {
	JID            JID
	Name           string
	Topic          string
	Picture        string
	Owner          JID
	Participants   []GroupParticipant
	Created        time.Time
	Description    string
	InviteLink     string
	IsAnnounce     bool
	IsLocked       bool
	IsParent       bool
	ParentGroup    JID
	LinkedGroups   []JID
}

// GroupParticipant represents a group participant
type GroupParticipant struct {
	JID      JID
	Role     GroupRole
	Added    time.Time
	Inviter  JID
}

// GroupRole represents a participant role
type GroupRole string

const (
	GroupRoleMember      GroupRole = "member"
	GroupRoleAdmin       GroupRole = "admin"
	GroupRoleSuperAdmin  GroupRole = "super_admin"
)

// ReceiptType represents receipt type
type ReceiptType string

const (
	ReceiptTypeDelivered ReceiptType = "delivered"
	ReceiptTypeRead      ReceiptType = "read"
	ReceiptTypePlayed    ReceiptType = "played"
)

// Receipt represents a message receipt
type Receipt struct {
	MessageID MessageID
	From      JID
	To        JID
	Type      ReceiptType
	Timestamp time.Time
}

// Presence represents user presence
type Presence struct {
	From      JID
	Type      PresenceType
	LastSeen  time.Time
}

// PresenceType represents presence type
type PresenceType string

const (
	PresenceAvailable   PresenceType = "available"
	PresenceUnavailable PresenceType = "unavailable"
	PresenceComposing   PresenceType = "composing"
	PresenceRecording   PresenceType = "recording"
)

// Typing represents typing indicator
type Typing struct {
	From    JID
	To      JID
	IsGroup bool
	Typing  bool
}

// QRCode represents a QR code for pairing
type QRCode struct {
	Code      string
	Timeout   time.Duration
	Refreshed time.Time
}

// PairingCode represents a pairing code
type PairingCode struct {
	Code      string
	Timeout   time.Duration
	Refreshed time.Time
}