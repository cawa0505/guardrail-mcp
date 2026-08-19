// Copyright 2026 jimmy. All rights reserved.
// Use of this source code is governed by a MIT-style license.
package token

import (
	"testing"
	"time"

	"github.com/cawa0505/guardrail-mcp/internal/state"
)

func TestHashProposalContent(t *testing.T) {
	h := HashProposalContent("hello")
	if len(h) != 64 {
		t.Errorf("expected 64 hex chars, got %d: %s", len(h), h)
	}
	// Same input should give same hash
	h2 := HashProposalContent("hello")
	if h != h2 {
		t.Errorf("expected same hash for same input, got %s vs %s", h, h2)
	}
}

func TestNewToken(t *testing.T) {
	tok, err := New("abc123", "/workspace", 0)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if tok == nil {
		t.Fatal("expected non-nil token")
	}
	if len(tok.ID) != 64 {
		t.Errorf("expected 64-char ID, got %d", len(tok.ID))
	}
	if tok.Bindings.ProposalHash != "abc123" {
		t.Errorf("expected proposal_hash abc123, got %s", tok.Bindings.ProposalHash)
	}
	if tok.Bindings.WorkspacePath != "/workspace" {
		t.Errorf("expected workspace /workspace, got %s", tok.Bindings.WorkspacePath)
	}
	if tok.Used || tok.Revoked {
		t.Error("new token should not be used or revoked")
	}
	expectedExpiry := tok.CreatedAt.Add(DefaultTTL)
	if !tok.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected expiry %v, got %v", expectedExpiry, tok.ExpiresAt)
	}
}

func TestNewToken_CustomTTL(t *testing.T) {
	tok, err := New("abc", "/ws", 5*time.Minute)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	expectedExpiry := tok.CreatedAt.Add(5 * time.Minute)
	if !tok.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected expiry %v, got %v", expectedExpiry, tok.ExpiresAt)
	}
}

func TestIsExpired_Fresh(t *testing.T) {
	tok := &state.CommitToken{
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	}
	if IsExpired(tok) {
		t.Error("fresh token should not be expired")
	}
}

func TestIsExpired_Past(t *testing.T) {
	tok := &state.CommitToken{
		CreatedAt: time.Now().UTC().Add(-31 * time.Minute),
		ExpiresAt: time.Now().UTC().Add(-1 * time.Minute),
	}
	if !IsExpired(tok) {
		t.Error("expired token should be expired")
	}
}

func TestIsValid(t *testing.T) {
	fresh := &state.CommitToken{
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	}
	if !IsValid(fresh) {
		t.Error("fresh token should be valid")
	}

	used := &state.CommitToken{
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
		Used:      true,
	}
	if IsValid(used) {
		t.Error("used token should not be valid")
	}

	revoked := &state.CommitToken{
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
		Revoked:   true,
	}
	if IsValid(revoked) {
		t.Error("revoked token should not be valid")
	}

	expired := &state.CommitToken{
		CreatedAt: time.Now().UTC().Add(-31 * time.Minute),
		ExpiresAt: time.Now().UTC().Add(-1 * time.Minute),
	}
	if IsValid(expired) {
		t.Error("expired token should not be valid")
	}
}

func TestConsume(t *testing.T) {
	tok := &state.CommitToken{
		ID:        "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	}
	if err := Consume(tok); err != nil {
		t.Fatalf("Consume() failed: %v", err)
	}
	if !tok.Used {
		t.Error("token should be marked used after Consume")
	}
	if tok.ConsumedAt == nil {
		t.Error("ConsumedAt should be set")
	}
	// Second consume should fail
	if err := Consume(tok); err == nil {
		t.Error("second Consume() should fail")
	}
}

func TestConsume_Revoked(t *testing.T) {
	tok := &state.CommitToken{
		ID:        "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
		Revoked:   true,
	}
	if err := Consume(tok); err == nil {
		t.Error("Consume() on revoked token should fail")
	}
}

func TestConsume_Expired(t *testing.T) {
	tok := &state.CommitToken{
		ID:        "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		CreatedAt: time.Now().UTC().Add(-31 * time.Minute),
		ExpiresAt: time.Now().UTC().Add(-1 * time.Minute),
	}
	if err := Consume(tok); err == nil {
		t.Error("Consume() on expired token should fail")
	}
}

func TestRevoke(t *testing.T) {
	tok := &state.CommitToken{
		ID:        "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	}
	if err := Revoke(tok); err != nil {
		t.Fatalf("Revoke() failed: %v", err)
	}
	if !tok.Revoked {
		t.Error("token should be marked revoked")
	}
	if tok.RevokedAt == nil {
		t.Error("RevokedAt should be set")
	}
	// Second revoke should fail
	if err := Revoke(tok); err == nil {
		t.Error("second Revoke() should fail")
	}
}

func TestRevoke_Consumed(t *testing.T) {
	tok := &state.CommitToken{
		ID:        "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
		Used:      true,
	}
	if err := Revoke(tok); err == nil {
		t.Error("Revoke() on consumed token should fail")
	}
}

func TestValidateBindings_Match(t *testing.T) {
	now := time.Now().UTC()
	tok := &state.CommitToken{
		ID:        "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		CreatedAt: now,
		ExpiresAt: now.Add(30 * time.Minute),
		Bindings: state.TokenBindings{
			ProposalHash:  "abc123",
			WorkspacePath: "/ws",
		},
	}
	if err := ValidateBindings(tok, "abc123", "/ws"); err != nil {
		t.Fatalf("ValidateBindings() should succeed: %v", err)
	}
}

func TestValidateBindings_Mismatch(t *testing.T) {
	now := time.Now().UTC()
	tok := &state.CommitToken{
		ID:        "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		CreatedAt: now,
		ExpiresAt: now.Add(30 * time.Minute),
		Bindings: state.TokenBindings{
			ProposalHash:  "abc123",
			WorkspacePath: "/ws",
		},
	}
	if err := ValidateBindings(tok, "xyz", "/ws"); err == nil {
		t.Error("ValidateBindings() should fail on proposal hash mismatch")
	}
	if err := ValidateBindings(tok, "abc123", "/other"); err == nil {
		t.Error("ValidateBindings() should fail on workspace path mismatch")
	}
}