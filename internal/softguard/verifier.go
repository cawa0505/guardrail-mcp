// Copyright 2026 jimmy Yen (cawa0505). All rights reserved.
// Use of this source code is governed by a MIT-style license.
package softguard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ── Types ──

// VerifierConfig defines one verifier from the config file.
type VerifierConfig struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	APIToken string `json:"api_token,omitempty"`
	Enabled  bool   `json:"enabled"`
	Required bool   `json:"required"`
}

// VerifierInput is sent to each verifier's HTTP endpoint.
type VerifierInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
	Patch    string `json:"patch,omitempty"`
	Phase    string `json:"phase"`
}

// VerifierResult is the outcome of one verifier check.
type VerifierResult struct {
	VerifierName string `json:"verifier_name"`
	Passed       bool   `json:"passed"`
	Required     bool   `json:"required"`
	Message      string `json:"message,omitempty"`
	Error        string `json:"error,omitempty"`
}

// ── Runner ──

// RunAll calls every enabled verifier and returns all results.
// A nil/empty config returns nil (no verification).
func RunAll(ctx context.Context, cfg *Config, input VerifierInput) []*VerifierResult {
	if cfg == nil {
		return nil
	}
	var results []*VerifierResult
	for _, vc := range cfg.Verifiers {
		if !vc.Enabled {
			continue
		}
		r := runOne(ctx, vc, input)
		results = append(results, r)
	}
	return results
}

// HasRequiredFailure checks whether any required verifier failed.
func HasRequiredFailure(results []*VerifierResult) *VerifierResult {
	for _, r := range results {
		if r.Required && !r.Passed {
			return r
		}
	}
	return nil
}

func runOne(ctx context.Context, vc VerifierConfig, input VerifierInput) *VerifierResult {
	body, err := json.Marshal(input)
	if err != nil {
		return &VerifierResult{
			VerifierName: vc.Name,
			Passed:       false,
			Error:        fmt.Sprintf("marshal input: %v", err),
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", vc.URL, bytes.NewReader(body))
	if err != nil {
		return &VerifierResult{
			VerifierName: vc.Name,
			Passed:       false,
			Error:        fmt.Sprintf("create request: %v", err),
		}
	}
	req.Header.Set("Content-Type", "application/json")
	if vc.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+vc.APIToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &VerifierResult{
			VerifierName: vc.Name,
			Passed:       false,
			Error:        fmt.Sprintf("http call: %v", err),
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &VerifierResult{
			VerifierName: vc.Name,
			Passed:       false,
			Error:        fmt.Sprintf("read response: %v", err),
		}
	}

	var vr struct {
		Passed  bool   `json:"passed"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &vr); err != nil {
		return &VerifierResult{
			VerifierName: vc.Name,
			Passed:       false,
			Error:        fmt.Sprintf("parse response: %v", err),
		}
	}

	return &VerifierResult{
		VerifierName: vc.Name,
		Passed:       vr.Passed,
		Message:      vr.Message,
	}
}