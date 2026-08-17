// Package binary implements the WhatsApp binary protocol (waBinary).
// Copyright (c) 2024 Soumen-Developer. All rights reserved.
// Licensed under MPL-2.0 (see LICENSE).
package binary

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

// Decoder decodes WhatsApp binary protocol nodes.
type Decoder struct {
	reader *bytes.Reader
	tokens *TokenMap
}

// NewDecoder creates a new decoder.
func NewDecoder(data []byte) *Decoder {
	return &Decoder{
		reader: bytes.NewReader(data),
		tokens: NewTokenMap(),
	}
}

// Decode decodes the next node from the stream.
func (d *Decoder) Decode() (*Node, error) {
	return d.readNode()
}

// readNode reads a single node.
func (d *Decoder) readNode() (*Node, error) {
	// Read tag
	tag, err := d.readTag()
	if err != nil {
		return nil, err
	}

	// Read attributes count
	attrCount, err := d.readListSize()
	if err != nil {
		return nil, err
	}

	node := NewNode(tag)
	node.Attrs = make(map[string]string)

	// Read attributes
	for i := 0; i < attrCount; i++ {
		key, err := d.readString()
		if err != nil {
			return nil, err
		}
		value, err := d.readString()
		if err != nil {
			return nil, err
		}
		node.Attrs[key] = value
	}

	// Read content or children
	nextByte, err := d.reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if nextByte == 0x00 {
		// Empty node
		return node, nil
	}

	// Put the byte back
	d.reader.UnreadByte()

	// Check if it's a list (children) or bytes (content)
	// Peek at the next byte to determine
	peek, err := d.reader.ReadByte()
	if err != nil {
		return nil, err
	}
	d.reader.UnreadByte()

	// If it's a list size marker (0xF8, 0xF9, 0xFA), it's children
	// Otherwise it's content bytes
	if peek == 0xF8 || peek == 0xF9 || peek == 0xFA {
		// List of children
		childCount, err := d.readListSize()
		if err != nil {
			return nil, err
		}
		node.Children = make([]*Node, 0, childCount)
		for i := 0; i < childCount; i++ {
			child, err := d.readNode()
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, child)
		}
	} else {
		// Content bytes
		d.reader.UnreadByte()
		content, err := d.readBytes()
		if err != nil {
			return nil, err
		}
		node.Content = content
	}

	return node, nil
}

// readTag reads a tag.
func (d *Decoder) readTag() (string, error) {
	token, err := d.readToken()
	if err != nil {
		return "", err
	}
	if token <= 0xFF {
		if s, ok := d.tokens.GetTagString(token); ok {
			return s, nil
		}
	}
	return d.readRawString()
}

// readString reads a string (dictionary token or raw).
func (d *Decoder) readString() (string, error) {
	token, err := d.readToken()
	if err != nil {
		return "", err
	}
	if token <= 0xFF {
		if s, ok := d.tokens.GetAttrString(token); ok {
			return s, nil
		}
	}
	if token == 0xFA { // JID
		return d.readJID()
	}
	d.reader.UnreadByte()
	return d.readRawString()
}

// readToken reads a token (single or double byte).
func (d *Decoder) readToken() (uint16, error) {
	b, err := d.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b == 0xFC {
		// Double byte token
		var token uint16
		err := binary.Read(d.reader, binary.BigEndian, &token)
		return token, err
	}
	return uint16(b), nil
}

// readRawString reads a raw string (bytes with length prefix).
func (d *Decoder) readRawString() (string, error) {
	data, err := d.readBytes()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readJID reads a JID.
func (d *Decoder) readJID() (string, error) {
	user, err := d.readBytes()
	if err != nil {
		return "", err
	}
	server, err := d.readBytes()
	if err != nil {
		return "", err
	}
	return string(user) + "@" + string(server), nil
}

// readBytes reads a byte slice with length prefix.
func (d *Decoder) readBytes() ([]byte, error) {
	size, err := d.readListSize()
	if err != nil {
		return nil, err
	}
	data := make([]byte, size)
	n, err := io.ReadFull(d.reader, data)
	if err != nil {
		return nil, err
	}
	if n != size {
		return nil, errors.New("unexpected EOF reading bytes")
	}
	return data, nil
}

// readListSize reads a list size.
func (d *Decoder) readListSize() (int, error) {
	b, err := d.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	switch b {
	case 0x00:
		return 0, nil
	case 0xF8:
		b, err := d.reader.ReadByte()
		return int(b), err
	case 0xF9:
		var size uint16
		err := binary.Read(d.reader, binary.BigEndian, &size)
		return int(size), err
	case 0xFA:
		var size uint32
		err := binary.Read(d.reader, binary.BigEndian, &size)
		return int(size), err
	default:
		// Not a list size marker - this is content
		d.reader.UnreadByte()
		return -1, nil
	}
}

// Reader provides a streaming decoder for large payloads.
type Reader struct {
	decoder *Decoder
}

// NewReader creates a new streaming reader.
func NewReader(data []byte) *Reader {
	return &Reader{
		decoder: NewDecoder(data),
	}
}

// Next returns the next node.
func (r *Reader) Next() (*Node, error) {
	return r.decoder.Decode()
}

// Err returns any error from the reader.
func (r *Reader) Err() error {
	return nil
}