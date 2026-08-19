// Copyright 2026 jimmy Yen (cawa0505). All rights reserved.
// Use of this source code is governed by a MIT-style license.
package graphify

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client launches Graphify as a subprocess and calls graphify_skeleton_extract.
type Client struct {
	cfg *Config
}

// NewClient creates a Client that uses the given config.
func NewClient(cfg *Config) *Client {
	return &Client{cfg: cfg}
}

// SkeletonResult holds the result of a skeleton_extract call.
type SkeletonResult struct {
	Text string // AST skeleton text (~300 tokens)
}

// SkeletonExtract calls graphify_skeleton_extract on the given file path and
// returns the compact AST skeleton.
//
// Returns an error if Graphify is not configured, the subprocess fails, or the
// call times out.
func (c *Client) SkeletonExtract(ctx context.Context, filePath string) (*SkeletonResult, error) {
	if c.cfg.BinaryPath == "" {
		return nil, fmt.Errorf("graphify binary not configured")
	}

	// Apply timeout from config if it is shorter than the parent context.
	ctx = c.withTimeout(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.cfg.BinaryPath)
	transport := &mcp.CommandTransport{Command: cmd, TerminateDuration: 5 * time.Second}
	client := mcp.NewClient(&mcp.Implementation{Name: "guardrail-graphify", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("graphify connect: %w", err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "graphify_skeleton_extract",
		Arguments: map[string]any{
			"path": filePath,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("graphify skeleton_extract: %w", err)
	}
	if res.IsError {
		return nil, fmt.Errorf("graphify skeleton_extract returned error")
	}

	text := extractText(res)
	return &SkeletonResult{Text: text}, nil
}

// withTimeout wraps ctx with the configured timeout if it is shorter than any
// existing deadline.
func (c *Client) withTimeout(ctx context.Context) context.Context {
	if c.cfg.Timeout <= 0 {
		return ctx
	}
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > c.cfg.Timeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		_ = cancel // used when the call returns
	}
	return ctx
}

// extractText concatenates all text content from a CallToolResult.
func extractText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(t.Text)
		}
	}
	return b.String()
}