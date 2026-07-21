// Package guardrail screens inbound chat requests for prompt-injection
// attempts. The Guardrail interface keeps the door open for model-based
// screeners; Phase 2 ships the heuristic pattern screener.
package guardrail

import "regexp"

// Modes for AGENTOS_GUARDRAILS_MODE.
const (
	ModeOff   = "off"
	ModeLog   = "log"
	ModeBlock = "block"
)

// ValidMode reports whether mode is one of off|log|block.
func ValidMode(mode string) bool {
	return mode == ModeOff || mode == ModeLog || mode == ModeBlock
}

// Verdict is the result of screening one request.
type Verdict struct {
	Flagged bool
	// Reason names the matched rule, e.g. "ignore-instructions".
	Reason string
}

// Guardrail screens the latest user message of a chat request.
type Guardrail interface {
	Screen(latestUserMessage string) Verdict
}

// rule pairs a human-readable name with its compiled pattern.
type rule struct {
	name    string
	pattern *regexp.Regexp
}

// HeuristicScreen flags well-known prompt-injection phrasings with
// case-insensitive regular expressions.
type HeuristicScreen struct {
	rules []rule
}

// NewHeuristicScreen builds the Phase 2 pattern screener.
func NewHeuristicScreen() *HeuristicScreen {
	return &HeuristicScreen{rules: []rule{
		{"ignore-instructions", regexp.MustCompile(`(?i)ignore\s+(?:(?:all|any)\s+)?(?:previous|above|all)\s+instructions`)},
		{"disregard-instructions", regexp.MustCompile(`(?i)disregard\s+(?:(?:all|any)\s+)?your\s+instructions`)},
		{"dan-jailbreak", regexp.MustCompile(`(?i)you\s+are\s+now\s+dan\b`)},
		{"system-prompt-exfiltration", regexp.MustCompile(`(?i)(?:reveal|print)\s+your\s+system\s+prompt`)},
		{"fake-system-header", regexp.MustCompile(`(?i)###\s*system`)},
		{"base64-blob", regexp.MustCompile(`[A-Za-z0-9+/=]{201,}`)},
	}}
}

// Screen checks the message against every rule; the first hit wins.
func (h *HeuristicScreen) Screen(latestUserMessage string) Verdict {
	for _, r := range h.rules {
		if r.pattern.MatchString(latestUserMessage) {
			return Verdict{Flagged: true, Reason: r.name}
		}
	}
	return Verdict{}
}
