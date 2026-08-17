// Package crypto provides cryptographic primitives for WhatsApp-Go.
// Copyright (c) 2024 Soumen-Developer. All rights reserved.
// Licensed under MPL-2.0 (see LICENSE).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// Key sizes
const (
	Curve25519KeySize   = 32
	Ed25519KeySize      = 32
	Ed25519SigSize      = 64
	AES256KeySize       = 32
	AES128KeySize       = 16
	AESIVSize           = 16
	HMACSHA256Size      = 32
	ChaChaPolyKeySize   = 32
	ChaChaPolyNonceSize = 12
	ChaChaPolyTagSize   = 16
)

// GenerateCurve25519KeyPair generates a new Curve25519 key pair.
func GenerateCurve25519KeyPair() (privateKey, publicKey []byte, err error) {
	privateKey = make([]byte, Curve25519KeySize)
	if _, err = rand.Read(privateKey); err != nil {
		return nil, nil, err
	}
	// Clamp private key for Curve25519
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	publicKey, err = curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, publicKey, nil
}

// Curve25519SharedSecret computes the shared secret using Curve25519.
func Curve25519SharedSecret(privateKey, peerPublicKey []byte) ([]byte, error) {
	if len(privateKey) != Curve25519KeySize || len(peerPublicKey) != Curve25519KeySize {
		return nil, errors.New("invalid key size")
	}
	return curve25519.X25519(privateKey, peerPublicKey)
}

// GenerateEd25519KeyPair generates a new Ed25519 key pair.
func GenerateEd25519KeyPair() (privateKey, publicKey []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv, pub, nil
}

// Ed25519Sign signs a message with Ed25519 private key.
func Ed25519Sign(privateKey, message []byte) ([]byte, error) {
	if len(privateKey) != Ed25519KeySize+Ed25519KeySize {
		// If only seed (32 bytes), expand
		if len(privateKey) == Ed25519KeySize {
			privateKey = ed25519.NewKeyFromSeed(privateKey)
		} else {
			return nil, errors.New("invalid private key size")
		}
	}
	return ed25519.Sign(privateKey, message), nil
}

// Ed25519Verify verifies an Ed25519 signature.
func Ed25519Verify(publicKey, message, signature []byte) bool {
	if len(publicKey) != Ed25519KeySize || len(signature) != Ed25519SigSize {
		return false
	}
	return ed25519.Verify(publicKey, message, signature)
}

// HKDFSHA256 derives keys using HKDF-SHA256.
func HKDFSHA256(secret, salt, info []byte, length int) ([]byte, error) {
	hkdfReader := hkdf.New(sha256.New, secret, salt, info)
	out := make([]byte, length)
	if _, err := hkdfReader.Read(out); err != nil {
		return nil, err
	}
	return out, nil
}

// HKDFExpand expands a key using HKDF-SHA256 (no salt).
func HKDFExpand(prk, info []byte, length int) ([]byte, error) {
	return HKDFSHA256(prk, nil, info, length)
}

// AES256GCMEncrypt encrypts plaintext using AES-256-GCM.
func AES256GCMEncrypt(key, nonce, plaintext, additionalData []byte) ([]byte, error) {
	if len(key) != AES256KeySize {
		return nil, errors.New("invalid key size: need 32 bytes")
	}
	if len(nonce) != 12 {
		return nil, errors.New("invalid nonce size: need 12 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, plaintext, additionalData), nil
}

// AES256GCMDecrypt decrypts ciphertext using AES-256-GCM.
func AES256GCMDecrypt(key, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	if len(key) != AES256KeySize {
		return nil, errors.New("invalid key size: need 32 bytes")
	}
	if len(nonce) != 12 {
		return nil, errors.New("invalid nonce size: need 12 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// AES256CBCEncrypt encrypts plaintext using AES-256-CBC.
func AES256CBCEncrypt(key, iv, plaintext []byte) ([]byte, error) {
	if len(key) != AES256KeySize {
		return nil, errors.New("invalid key size: need 32 bytes")
	}
	if len(iv) != AESIVSize {
		return nil, errors.New("invalid IV size: need 16 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// PKCS7 padding
	padding := AESIVSize - (len(plaintext) % AESIVSize)
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)
	return ciphertext, nil
}

// AES256CBCDecrypt decrypts ciphertext using AES-256-CBC.
func AES256CBCDecrypt(key, iv, ciphertext []byte) ([]byte, error) {
	if len(key) != AES256KeySize {
		return nil, errors.New("invalid key size: need 32 bytes")
	}
	if len(iv) != AESIVSize {
		return nil, errors.New("invalid IV size: need 16 bytes")
	}
	if len(ciphertext)%AESIVSize != 0 {
		return nil, errors.New("ciphertext not multiple of block size")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)
	// Remove PKCS7 padding
	padding := int(plaintext[len(plaintext)-1])
	if padding > AESIVSize || padding == 0 {
		return nil, errors.New("invalid padding")
	}
	for i := len(plaintext) - padding; i < len(plaintext); i++ {
		if plaintext[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}
	return plaintext[:len(plaintext)-padding], nil
}

// HMACSHA256 computes HMAC-SHA256.
func HMACSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// VerifyHMACSHA256 verifies HMAC-SHA256.
func VerifyHMACSHA256(key, data, expectedMAC []byte) bool {
	actual := HMACSHA256(key, data)
	return hmac.Equal(actual, expectedMAC)
}

// ChaChaPolyEncrypt encrypts using ChaCha20-Poly1305.
func ChaChaPolyEncrypt(key, nonce, plaintext, additionalData []byte) ([]byte, error) {
	if len(key) != ChaChaPolyKeySize {
		return nil, errors.New("invalid key size: need 32 bytes")
	}
	if len(nonce) != ChaChaPolyNonceSize {
		return nil, errors.New("invalid nonce size: need 12 bytes")
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, plaintext, additionalData), nil
}

// ChaChaPolyDecrypt decrypts using ChaCha20-Poly1305.
func ChaChaPolyDecrypt(key, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	if len(key) != ChaChaPolyKeySize {
		return nil, errors.New("invalid key size: need 32 bytes")
	}
	if len(nonce) != ChaChaPolyNonceSize {
		return nil, errors.New("invalid nonce size: need 12 bytes")
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// RandomBytes generates cryptographically secure random bytes.
func RandomBytes(length int) ([]byte, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// AppendU64 appends a uint64 in little-endian format.
func AppendU64(buf []byte, v uint64) []byte {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	return append(buf, b[:]...)
}

// ReadU64 reads a uint64 in little-endian format.
func ReadU64(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b)
}