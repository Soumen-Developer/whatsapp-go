// Package noise implements the Noise Protocol Framework (IK pattern) for WhatsApp.
// Copyright (c) 2024 Soumen-Developer. All rights reserved.
// Licensed under MPL-2.0 (see LICENSE).
package noise

import (
	"errors"

	"github.com/Soumen-Developer/whatsapp-go/crypto"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
	"crypto/sha256"
)

// HandshakeState represents the Noise handshake state machine.
type HandshakeState struct {
	// Pattern: IK
	// Initiator: static keypair (s), ephemeral keypair (e)
	// Responder: static keypair (s), ephemeral keypair (e)
	
	// Role
	initiator bool
	
	// Static keypairs
	localStatic  *[32]byte // our static private key
	localStaticPub *[32]byte // our static public key
	remoteStaticPub *[32]byte // peer's static public key
	
	// Ephemeral keypairs
	localEphemeral  *[32]byte
	localEphemeralPub *[32]byte
	remoteEphemeralPub *[32]byte
	
	// Handshake hash
	h [32]byte
	
	// Cipher state
	cipherState *CipherState
	
	// Symmetric state
	symmetricState *SymmetricState
	
	// Message patterns for IK
	// Initiator: e, es, s, ss
	// Responder: e, ee, se, ss
	messagePatterns [][]string
	currentPattern  int
	
	// Prologue (WhatsApp-specific)
	prologue []byte
	
	// Completed
	completed bool
}

// NewInitiatorState creates a new handshake state for the initiator.
func NewInitiatorState(prologue []byte, localStatic, remoteStaticPub *[32]byte) *HandshakeState {
	hs := &HandshakeState{
		initiator:       true,
		localStatic:     localStatic,
		localStaticPub:  publicKeyFromPrivate(localStatic),
		remoteStaticPub: remoteStaticPub,
		prologue:        prologue,
	}
	hs.initialize()
	return hs
}

// NewResponderState creates a new handshake state for the responder.
func NewResponderState(prologue []byte, localStatic, remoteStaticPub *[32]byte) *HandshakeState {
	hs := &HandshakeState{
		initiator:       false,
		localStatic:     localStatic,
		localStaticPub:  publicKeyFromPrivate(localStatic),
		remoteStaticPub: remoteStaticPub,
		prologue:        prologue,
	}
	hs.initialize()
	return hs
}

func (hs *HandshakeState) initialize() {
	// Initialize symmetric state with protocol name
	protocolName := "Noise_IK_25519_ChaChaPoly_SHA256"
	hs.symmetricState = NewSymmetricState([]byte(protocolName))
	
	// Mix prologue into hash
	hs.symmetricState.MixHash(hs.prologue)
	
	// Mix responder's static public key into hash (for IK pattern)
	hs.symmetricState.MixHash(hs.remoteStaticPub[:])
	
	// Generate local ephemeral keypair
	hs.localEphemeral = crypto.GenerateCurve25519PrivateKey()
	hs.localEphemeralPub = publicKeyFromPrivate(hs.localEphemeral)
	
	// Set up message patterns for IK
	if hs.initiator {
		// Initiator sends: e, es, s, ss
		hs.messagePatterns = [][]string{
			{"e"},
			{"es"},
			{"s", "es"},
			{"ss"},
		}
	} else {
		// Responder sends: e, ee, se, ss
		hs.messagePatterns = [][]string{
			{"e"},
			{"ee", "se"},
			{"ss"},
		}
	}
	hs.currentPattern = 0
	hs.cipherState = NewCipherState()
}

// WriteMessage writes the next handshake message.
func (hs *HandshakeState) WriteMessage(payload []byte) ([]byte, error) {
	if hs.completed {
		return nil, errors.New("handshake already completed")
	}
	if hs.currentPattern >= len(hs.messagePatterns) {
		return nil, errors.New("no more message patterns")
	}
	
	pattern := hs.messagePatterns[hs.currentPattern]
	
	// Build the message node
	message := make([]byte, 0, 1024)
	
	for _, token := range pattern {
		switch token {
		case "e":
			// Write our ephemeral public key
			message = append(message, hs.localEphemeralPub[:]...)
			hs.symmetricState.MixHash(hs.localEphemeralPub[:])
			
		case "es":
			// DH(local_e, remote_s)
			dh := crypto.Curve25519DH(hs.localEphemeral, hs.remoteStaticPub)
			hs.symmetricState.MixKey(dh[:])
			
		case "s":
			// Encrypt and write our static public key
			encrypted := hs.symmetricState.EncryptAndHash(hs.localStaticPub[:])
			message = append(message, encrypted...)
			
		case "ee":
			// DH(local_e, remote_e)
			if hs.remoteEphemeralPub == nil {
				return nil, errors.New("remote ephemeral public key not set")
			}
			dh := crypto.Curve25519DH(hs.localEphemeral, hs.remoteEphemeralPub)
			hs.symmetricState.MixKey(dh[:])
			
		case "se":
			if hs.initiator {
				// DH(local_s, remote_e)
				dh := crypto.Curve25519DH(hs.localStatic, hs.remoteEphemeralPub)
				hs.symmetricState.MixKey(dh[:])
			} else {
				// DH(remote_s, local_e)
				dh := crypto.Curve25519DH(hs.remoteStaticPub, hs.localEphemeral)
				hs.symmetricState.MixKey(dh[:])
			}
			
		case "ss":
			// DH(local_s, remote_s)
			dh := crypto.Curve25519DH(hs.localStatic, hs.remoteStaticPub)
			hs.symmetricState.MixKey(dh[:])
		}
	}
	
	// Encrypt payload
	ciphertext := hs.symmetricState.EncryptAndHash(payload)
	message = append(message, ciphertext...)
	
	hs.currentPattern++
	
	// Check if handshake is complete
	if hs.currentPattern >= len(hs.messagePatterns) {
		hs.completed = true
		// Split into two cipher states for transport
		hs.cipherState = hs.symmetricState.Split()
	}
	
	return message, nil
}

// ReadMessage reads and processes a handshake message.
func (hs *HandshakeState) ReadMessage(message []byte) ([]byte, error) {
	if hs.completed {
		return nil, errors.New("handshake already completed")
	}
	if hs.currentPattern >= len(hs.messagePatterns) {
		return nil, errors.New("no more message patterns")
	}
	
	pattern := hs.messagePatterns[hs.currentPattern]
	offset := 0
	
	for _, token := range pattern {
		switch token {
		case "e":
			// Read remote ephemeral public key
			if offset+32 > len(message) {
				return nil, errors.New("message too short for ephemeral key")
			}
			var remoteEphemeralPub [32]byte
			copy(remoteEphemeralPub[:], message[offset:offset+32])
			hs.remoteEphemeralPub = &remoteEphemeralPub
			offset += 32
			hs.symmetricState.MixHash(hs.remoteEphemeralPub[:])
			
		case "es":
			// DH(local_e, remote_s) - already have remote static
			dh := crypto.Curve25519DH(hs.localEphemeral, hs.remoteStaticPub)
			hs.symmetricState.MixKey(dh[:])
			
		case "s":
			// Read and decrypt remote static public key
			if offset+48 > len(message) { // 32 bytes + 16 byte tag
				return nil, errors.New("message too short for encrypted static key")
			}
			ciphertext := message[offset : offset+48]
			offset += 48
			plaintext, err := hs.symmetricState.DecryptAndHash(ciphertext)
			if err != nil {
				return nil, err
			}
			if len(plaintext) != 32 {
				return nil, errors.New("invalid static public key length")
			}
			var remoteStaticPub [32]byte
			copy(remoteStaticPub[:], plaintext)
			hs.remoteStaticPub = &remoteStaticPub
			hs.symmetricState.MixHash(plaintext)
			
		case "ee":
			if hs.remoteEphemeralPub == nil {
				return nil, errors.New("remote ephemeral public key not set")
			}
			dh := crypto.Curve25519DH(hs.localEphemeral, hs.remoteEphemeralPub)
			hs.symmetricState.MixKey(dh[:])
			
		case "se":
			if hs.initiator {
				dh := crypto.Curve25519DH(hs.localStatic, hs.remoteEphemeralPub)
				hs.symmetricState.MixKey(dh[:])
			} else {
				dh := crypto.Curve25519DH(hs.remoteStaticPub, hs.localEphemeral)
				hs.symmetricState.MixKey(dh[:])
			}
			
		case "ss":
			dh := crypto.Curve25519DH(hs.localStatic, hs.remoteStaticPub)
			hs.symmetricState.MixKey(dh[:])
		}
	}
	
	// Decrypt payload
	if offset >= len(message) {
		return nil, errors.New("no payload in message")
	}
	ciphertext := message[offset:]
	payload, err := hs.symmetricState.DecryptAndHash(ciphertext)
	if err != nil {
		return nil, err
	}
	
	hs.currentPattern++
	
	if hs.currentPattern >= len(hs.messagePatterns) {
		hs.completed = true
		hs.cipherState = hs.symmetricState.Split()
	}
	
	return payload, nil
}

// IsCompleted returns true if handshake is complete.
func (hs *HandshakeState) IsCompleted() bool {
	return hs.completed
}

// GetCipherState returns the transport cipher state.
func (hs *HandshakeState) GetCipherState() *CipherState {
	return hs.cipherState
}

// GetHandshakeHash returns the final handshake hash.
func (hs *HandshakeState) GetHandshakeHash() [32]byte {
	return hs.symmetricState.GetHash()
}

// publicKeyFromPrivate derives public key from private key.
func publicKeyFromPrivate(private *[32]byte) *[32]byte {
	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, private)
	return &pub
}

// CipherState handles encryption/decryption for transport.
type CipherState struct {
	key  [32]byte
	nonce uint64
	hasKey bool
}

// NewCipherState creates a new cipher state.
func NewCipherState() *CipherState {
	return &CipherState{}
}

// InitializeKey sets the cipher key.
func (cs *CipherState) InitializeKey(key [32]byte) {
	cs.key = key
	cs.nonce = 0
	cs.hasKey = true
}

// HasKey returns true if cipher has a key.
func (cs *CipherState) HasKey() bool {
	return cs.hasKey
}

// Encrypt encrypts plaintext with ChaCha20-Poly1305.
func (cs *CipherState) Encrypt(plaintext, additionalData []byte) ([]byte, error) {
	if !cs.hasKey {
		return nil, errors.New("cipher state not initialized")
	}
	
	aead, err := chacha20poly1305.NewX(cs.key[:])
	if err != nil {
		return nil, err
	}
	
	var nonce [12]byte
	// Noise uses 12-byte nonce: first 4 bytes zero, last 8 bytes = big-endian nonce counter
	for i := 0; i < 8; i++ {
		nonce[4+i] = byte(cs.nonce >> (56 - i*8))
	}
	
	ciphertext := aead.Seal(nil, nonce[:], plaintext, additionalData)
	cs.nonce++
	return ciphertext, nil
}

// Decrypt decrypts ciphertext with ChaCha20-Poly1305.
func (cs *CipherState) Decrypt(ciphertext, additionalData []byte) ([]byte, error) {
	if !cs.hasKey {
		return nil, errors.New("cipher state not initialized")
	}
	
	aead, err := chacha20poly1305.NewX(cs.key[:])
	if err != nil {
		return nil, err
	}
	
	var nonce [12]byte
	for i := 0; i < 8; i++ {
		nonce[4+i] = byte(cs.nonce >> (56 - i*8))
	}
	
	plaintext, err := aead.Open(nil, nonce[:], ciphertext, additionalData)
	if err != nil {
		return nil, err
	}
	cs.nonce++
	return plaintext, nil
}

// GetNonce returns current nonce.
func (cs *CipherState) GetNonce() uint64 {
	return cs.nonce
}

// SetNonce sets the nonce.
func (cs *CipherState) SetNonce(nonce uint64) {
	cs.nonce = nonce
}

// SymmetricState manages the symmetric crypto state during handshake.
type SymmetricState struct {
	ck [32]byte // chaining key
	h  [32]byte // handshake hash
	tempKey [32]byte // temporary key from MixKey
	initialized bool
}

// NewSymmetricState creates a new symmetric state.
func NewSymmetricState(protocolName []byte) *SymmetricState {
	ss := &SymmetricState{}
	// h = SHA256(protocol_name)
	h := sha256.New()
	h.Write(protocolName)
	copy(ss.h[:], h.Sum(nil))
	// ck = h
	ss.ck = ss.h
	ss.initialized = true
	return ss
}

// MixHash mixes data into the handshake hash.
func (ss *SymmetricState) MixHash(data []byte) {
	h := sha256.New()
	h.Write(ss.h[:])
	h.Write(data)
	copy(ss.h[:], h.Sum(nil))
}

// MixKey mixes input key material into the chaining key.
func (ss *SymmetricState) MixKey(inputKeyMaterial []byte) {
	// ck, temp = HKDF(ck, input_key_material, "", 2)
	hkdfExtract := hkdf.New(sha256.New, inputKeyMaterial, ss.ck[:], nil)
	temp := make([]byte, 64)
	hkdfExtract.Read(temp)
	copy(ss.ck[:], temp[:32])
	copy(ss.tempKey[:], temp[32:])
}

// EncryptAndHash encrypts plaintext and mixes ciphertext into hash.
func (ss *SymmetricState) EncryptAndHash(plaintext []byte) []byte {
	if !ss.initialized {
		// No key yet, just mix plaintext into hash
		ss.MixHash(plaintext)
		return plaintext
	}
	
	// Use temp key from MixKey
	ciphertext, _ := crypto.ChaChaPolyEncrypt(ss.tempKey[:], make([]byte, 12), plaintext, ss.h[:])
	ss.MixHash(ciphertext)
	return ciphertext
}

// DecryptAndHash decrypts ciphertext and mixes it into hash.
func (ss *SymmetricState) DecryptAndHash(ciphertext []byte) ([]byte, error) {
	if !ss.initialized {
		ss.MixHash(ciphertext)
		return ciphertext, nil
	}
	
	plaintext, err := crypto.ChaChaPolyDecrypt(ss.tempKey[:], make([]byte, 12), ciphertext, ss.h[:])
	if err != nil {
		return nil, err
	}
	ss.MixHash(ciphertext)
	return plaintext, nil
}

// Split splits the symmetric state into two cipher states for transport.
func (ss *SymmetricState) Split() *CipherState {
	// k1, k2 = HKDF(ck, "", 2)
	hkdfExtract := hkdf.New(sha256.New, nil, ss.ck[:], []byte(""))
	keys := make([]byte, 64)
	hkdfExtract.Read(keys)
	
	cs1 := NewCipherState()
	cs1.InitializeKey(*(*[32]byte)(keys[:32]))
	
	cs2 := NewCipherState()
	cs2.InitializeKey(*(*[32]byte)(keys[32:]))
	
	// Return first cipher state (initiator gets cs1, responder gets cs2)
	return cs1
}

// GetHash returns the handshake hash.
func (ss *SymmetricState) GetHash() [32]byte {
	return ss.h
}

// GetChainingKey returns the chaining key.
func (ss *SymmetricState) GetChainingKey() [32]byte {
	return ss.ck
}