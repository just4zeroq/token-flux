// Package rtk implements the Reduced Token Kit — compresses tool_result content
// before sending to LLM providers, saving 20-40% of input tokens.
package rtk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Config for RTK compression.
type Config struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"` // "compress" | "summary" | "auto"
}

// CompressMessages applies RTK compression to tool results in a request body.
func CompressMessages(body []byte, cfg Config) []byte {
	if !cfg.Enabled {
		return body
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}

	messages, ok := req["messages"].([]any)
	if !ok {
		return body
	}

	changed := false
	for i, msg := range messages {
		m, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role != "tool" {
			continue
		}
		content, ok := m["content"].(string)
		if !ok || content == "" {
			continue
		}
		compressed := compressContent(content, cfg.Mode)
		if compressed != content {
			m["content"] = compressed
			messages[i] = m
			changed = true
		}
	}

	if !changed {
		return body
	}
	req["messages"] = messages
	out, _ := json.Marshal(req)
	return out
}

func compressContent(s string, mode string) string {
	if len(s) < 200 {
		return s
	}
	switch mode {
	case "compress":
		return applyCompression(s)
	case "summary":
		return applySummary(s)
	case "auto":
		fallthrough
	default:
		if isDiffContent(s) || isDirectoryListing(s) {
			return applyCompression(s)
		}
		return s
	}
}

var (
	diffHeader = regexp.MustCompile(`(?m)^diff --git a/`)
	hunkHeader = regexp.MustCompile(`(?m)^@@ -\d+,\d+ \+\d+,\d+ @@`)
	lsEntry    = regexp.MustCompile(`(?m)^[drwx-]{10}\s+\d+\s+\S+\s+\S+`)
	treeLine   = regexp.MustCompile(`(?m)^│?\s*├── |^│?\s*└── |^── `)
)

func isDiffContent(s string) bool {
	return diffHeader.MatchString(s) || hunkHeader.MatchString(s)
}

func isDirectoryListing(s string) bool {
	return lsEntry.MatchString(s) || treeLine.MatchString(s)
}

func applyCompression(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= 50 {
		return s
	}
	keep := 20
	var buf bytes.Buffer
	for i, line := range lines {
		if i < keep || i >= len(lines)-keep {
			buf.WriteString(line)
			buf.WriteByte('\n')
		} else if i == keep {
			removed := len(lines) - keep*2
			buf.WriteString(fmt.Sprintf("// ... %d lines compressed by RTK ...\n", removed))
		}
	}
	return strings.TrimSpace(buf.String())
}

func applySummary(s string) string {
	lines := strings.Split(s, "\n")
	return fmt.Sprintf("// RTK Summary: %d lines, %d words, %d chars\n// First: %s\n// Last: %s",
		len(lines), len(strings.Fields(s)), len(s),
		trimLine(lines, 0), trimLine(lines, len(lines)-1))
}

func trimLine(lines []string, idx int) string {
	if idx < 0 || idx >= len(lines) {
		return ""
	}
	s := lines[idx]
	if len(s) > 100 {
		return s[:100] + "..."
	}
	return s
}
