// Package binary implements the WhatsApp binary protocol (waBinary).
// Copyright (c) 2024 Soumen-Developer. All rights reserved.
// Licensed under MPL-2.0 (see LICENSE).
package binary

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// Encoder encodes WhatsApp binary protocol nodes.
type Encoder struct {
	buf    *bytes.Buffer
	tokens *TokenMap
}

// NewEncoder creates a new encoder.
func NewEncoder() *Encoder {
	return &Encoder{
		buf:    new(bytes.Buffer),
		tokens: NewTokenMap(),
	}
}

// Encode encodes a node to bytes.
func (e *Encoder) Encode(node *Node) ([]byte, error) {
	e.buf.Reset()
	if err := e.writeNode(node); err != nil {
		return nil, err
	}
	return e.buf.Bytes(), nil
}

// writeNode writes a single node.
func (e *Encoder) writeNode(node *Node) error {
	// Write tag
	if err := e.writeTag(node.Tag); err != nil {
		return err
	}

	// Write attributes count
	attrCount := len(node.Attrs)
	if err := e.writeListSize(attrCount); err != nil {
		return err
	}

	// Write attributes
	for key, value := range node.Attrs {
		if err := e.writeString(key); err != nil {
			return err
		}
		if err := e.writeString(value); err != nil {
			return err
		}
	}

	// Write content or children
	if len(node.Content) > 0 {
		// Leaf node with content
		if err := e.writeBytes(node.Content); err != nil {
			return err
		}
	} else if len(node.Children) > 0 {
		// List node with children
		if err := e.writeListSize(len(node.Children)); err != nil {
			return err
		}
		for _, child := range node.Children {
			if err := e.writeNode(child); err != nil {
				return err
			}
		}
	} else {
		// Empty node - write empty list marker
		e.buf.WriteByte(0x00)
	}

	return nil
}

// writeTag writes a tag (dictionary token or string).
func (e *Encoder) writeTag(tag string) error {
	if token, ok := e.tokens.GetTagToken(tag); ok {
		return e.writeToken(token)
	}
	return e.writeString(tag)
}

// writeString writes a string (dictionary token or raw string).
func (e *Encoder) writeString(s string) error {
	if token, ok := e.tokens.GetToken(s); ok {
		return e.writeToken(token)
	}
	return e.writeRawString(s)
}

// writeToken writes a dictionary token.
func (e *Encoder) writeToken(token uint16) error {
	if token <= 0xFF {
		// Single byte token
		e.buf.WriteByte(byte(token))
	} else {
		// Two byte token
		e.buf.WriteByte(0xFC)
		binary.Write(e.buf, binary.BigEndian, token)
	}
	return nil
}

// writeRawString writes a raw string (not in dictionary).
func (e *Encoder) writeRawString(s string) error {
	// Write as JID if it contains @
	if len(s) > 0 && bytes.Contains([]byte(s), []byte("@")) {
		return e.writeJID(s)
	}
	
	// Write as raw bytes
	return e.writeBytes([]byte(s))
}

// writeJID writes a JID.
func (e *Encoder) writeJID(jid string) error {
	e.buf.WriteByte(0xFA) // JID tag
	parts := bytes.Split([]byte(jid), []byte("@"))
	if len(parts) != 2 {
		return errors.New("invalid JID format")
	}
	user := parts[0]
	server := parts[1]
	
	// Write user part
	if err := e.writeBytes(user); err != nil {
		return err
	}
	// Write server part
	return e.writeBytes(server)
}

// writeBytes writes a byte slice with length prefix.
func (e *Encoder) writeBytes(data []byte) error {
	if err := e.writeListSize(len(data)); err != nil {
		return err
	}
	e.buf.Write(data)
	return nil
}

// writeListSize writes a list size (number of items or bytes).
func (e *Encoder) writeListSize(size int) error {
	switch {
	case size == 0:
		e.buf.WriteByte(0x00)
	case size <= 255:
		e.buf.WriteByte(0xF8)
		e.buf.WriteByte(byte(size))
	case size <= 65535:
		e.buf.WriteByte(0xF9)
		binary.Write(e.buf, binary.BigEndian, uint16(size))
	default:
		e.buf.WriteByte(0xFA)
		binary.Write(e.buf, binary.BigEndian, uint32(size))
	}
	return nil
}

// Node represents a node in the WhatsApp binary protocol tree.
type Node struct {
	Tag       string
	Attrs     map[string]string
	Content   []byte
	Children  []*Node
}

// NewNode creates a new node with the given tag.
func NewNode(tag string) *Node {
	return &Node{
		Tag:      tag,
		Attrs:    make(map[string]string),
		Children: make([]*Node, 0),
	}
}

// NewNodeWithContent creates a new node with tag and content.
func NewNodeWithContent(tag string, content []byte) *Node {
	return &Node{
		Tag:      tag,
		Attrs:    make(map[string]string),
		Content:  content,
		Children: make([]*Node, 0),
	}
}

// NewNodeWithAttrs creates a new node with tag and attributes.
func NewNodeWithAttrs(tag string, attrs map[string]string) *Node {
	return &Node{
		Tag:      tag,
		Attrs:    attrs,
		Children: make([]*Node, 0),
	}
}

// AddChild adds a child node.
func (n *Node) AddChild(child *Node) *Node {
	n.Children = append(n.Children, child)
	return n
}

// AddChildTag adds a child node with just a tag.
func (n *Node) AddChildTag(tag string) *Node {
	child := NewNode(tag)
	n.Children = append(n.Children, child)
	return child
}

// SetAttr sets an attribute.
func (n *Node) SetAttr(key, value string) *Node {
	n.Attrs[key] = value
	return n
}

// GetAttr gets an attribute value.
func (n *Node) GetAttr(key string) (string, bool) {
	v, ok := n.Attrs[key]
	return v, ok
}

// GetChild gets the first child with the given tag.
func (n *Node) GetChild(tag string) *Node {
	for _, child := range n.Children {
		if child.Tag == tag {
			return child
		}
	}
	return nil
}

// GetChildren gets all children with the given tag.
func (n *Node) GetChildren(tag string) []*Node {
	var result []*Node
	for _, child := range n.Children {
		if child.Tag == tag {
			result = append(result, child)
		}
	}
	return result
}

// GetContentString returns content as string.
func (n *Node) GetContentString() string {
	return string(n.Content)
}