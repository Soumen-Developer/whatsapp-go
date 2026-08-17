# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial project structure and module setup
- JID (WhatsApp ID) implementation with full type support
- Binary protocol (waBinary) encoder/decoder foundation
- Noise IK handshake framework
- Signal Protocol (X3DH + Double Ratchet) foundation
- WebSocket connection manager with framing
- SQLite session storage interface
- QR code pairing flow
- Passkey (WebAuthn) pairing support
- Message send/receive infrastructure
- Media upload/download with encryption
- Group management (create, participants, settings)
- Community support (announcement groups, linked groups)
- App State Sync (LTHash, contacts, settings, privacy)
- Newsletter support (subscribe, messages, reactions)
- Call signaling support
- Business features (catalog, quick replies)
- Status (send/view)
- Broadcast lists
- Device management

### Changed
- N/A

### Deprecated
- N/A

### Removed
- N/A

### Fixed
- N/A

### Security
- N/A

## [v0.1.0] - 2024-01-15

### Added
- **Phase 0**: Foundation
  - Module initialization (`github.com/Soumen-Developer/whatsapp-go`)
  - JID implementation (User, Group, Broadcast, Newsletter, LID, Bot)
  - Binary protocol Node structure, encoder, decoder
  - Crypto primitives (Curve25519, Ed25519, HKDF, AES-GCM)
  - Event type definitions
  - Comprehensive README with architecture diagrams
  - MPL-2.0 license with proper copyright
  - Makefile with test, lint, bench, build targets
  - golangci-lint configuration
  - GitHub Actions CI/CD workflows

---

## Release Checklist

For each release:

- [ ] Update version in `go.mod` (if needed)
- [ ] Update CHANGELOG.md with release notes
- [ ] Run `make check` (fmt, vet, lint, test)
- [ ] Run `make test-race`
- [ ] Create and push tag: `git tag v0.x.x && git push origin v0.x.x`
- [ ] GitHub Actions will build and create release
- [ ] Verify release artifacts on GitHub Releases page
- [ ] Announce in discussions/social media