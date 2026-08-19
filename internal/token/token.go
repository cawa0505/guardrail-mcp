// Copyright 2026 jimmy. All rights reserved.
// Use of this source code is governed by a MIT-style license.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cawa0505/guardrail-mcp/internal/state"
)

// DefaultTTL is the default token time-to-live.
const DefaultTTL = 30 * time.Minute

// ── Token Generation ──

// generateTokenID creates a cryptographically random hex token ID (32 bytes → 64 hex chars).
func generateTokenID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashProposalContent returns the SHA-256 hex digest of the given content.
func HashProposalContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// GitHeadRevision returns the current git HEAD revision hash.
func GitHeadRevision() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// New creates a new commit token bound to the given proposal and workspace.
// ttl specifies the token lifetime; zero means DefaultTTL.
func New(proposalHash, workspacePath string, ttl time.Duration) (*state.CommitToken, error) {
	id, err := generateTokenID()
	if err != nil {
		return nil, err
	}

	if ttl <= 0 {
		ttl = DefaultTTL
	}

	now := time.Now().UTC()
	rev := GitHeadRevision()

	return &state.CommitToken{
		ID:        id,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		Bindings: state.TokenBindings{
			ProposalHash:  proposalHash,
			WorkspacePath: workspacePath,
			Revision:      rev,
		},
	}, nil
}

// ── Validation ──

// IsExpired returns true if the token's expiration time has passed.
func IsExpired(t *state.CommitToken) bool {
	return time.Now().UTC().After(t.ExpiresAt)
}

// IsValid returns true if the token is usable (not expired, not used, not revoked).
func IsValid(t *state.CommitToken) bool {
	return !IsExpired(t) && !t.Used && !t.Revoked
}

// ValidateBindings checks that the given bindings match the token's bindings.
func ValidateBindings(t *state.CommitToken, proposalHash, workspacePath string) error {
	if t.Bindings.ProposalHash != "" && t.Bindings.ProposalHash != proposalHash {
		return fmt.Errorf("commit token: proposal hash mismatch")
	}
	if t.Bindings.WorkspacePath != "" && t.Bindings.WorkspacePath != workspacePath {
		return fmt.Errorf("commit token: workspace path mismatch")
	}
	if t.Bindings.Revision != "" {
		currentRev := GitHeadRevision()
		if currentRev != "" && currentRev != t.Bindings.Revision {
			return fmt.Errorf("commit token: git revision mismatch (token: %s, current: %s)", t.Bindings.Revision, currentRev)
		}
	}
	return nil
}

// ── Lifecycle ──

// Consume marks the token as used. Returns an error if the token is already
// consumed, revoked, or expired.
func Consume(t *state.CommitToken) error {
	if t.Used {
		return fmt.Errorf("commit token %s: already consumed", t.ID[:12])
	}
	if t.Revoked {
		return fmt.Errorf("commit token %s: revoked", t.ID[:12])
	}
	if IsExpired(t) {
		return fmt.Errorf("commit token %s: expired", t.ID[:12])
	}
	now := time.Now().UTC()
	t.Used = true
	t.ConsumedAt = &now
	return nil
}

// Revoke marks the token as revoked. Returns an error if already revoked or consumed.
func Revoke(t *state.CommitToken) error {
	if t.Revoked {
		return fmt.Errorf("commit token %s: already revoked", t.ID[:12])
	}
	if t.Used {
		return fmt.Errorf("commit token %s: already consumed, cannot revoke", t.ID[:12])
	}
	now := time.Now().UTC()
	t.Revoked = true
	t.RevokedAt = &now
	return nil
}