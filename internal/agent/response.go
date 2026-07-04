// internal/agent/response.go
package agent

import "encoding/json"

// Usage is token/cost accounting for one or more agent CLI calls.
type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CacheRead    int     `json:"cache_read_input_tokens"`
	CacheWrite   int     `json:"cache_creation_input_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// Add accumulates v into u; a nil v is a no-op.
func (u *Usage) Add(v *Usage) {
	if v == nil {
		return
	}
	u.InputTokens += v.InputTokens
	u.OutputTokens += v.OutputTokens
	u.CacheRead += v.CacheRead
	u.CacheWrite += v.CacheWrite
	u.CostUSD += v.CostUSD
}

// TotalIn is everything sent to the model: fresh input plus cache reads and
// cache writes. This is the honest "what went into this" number.
func (u Usage) TotalIn() int { return u.InputTokens + u.CacheRead + u.CacheWrite }

// Response is one agent CLI reply. Usage is nil when the CLI doesn't report it.
type Response struct {
	Text  string
	Usage *Usage
}

type claudeEnvelope struct {
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

// ParseClaudeJSON unwraps the `claude -p --output-format json` envelope.
// Anything that doesn't look like the envelope (including an empty result,
// e.g. from a future CLI format drift) falls back to raw passthrough so a
// decision can still be extracted or retried (ADR-0001).
func ParseClaudeJSON(raw string) Response {
	var env claudeEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil || env.Result == "" {
		return Response{Text: raw}
	}
	return Response{Text: env.Result, Usage: &Usage{
		InputTokens:  env.Usage.InputTokens,
		OutputTokens: env.Usage.OutputTokens,
		CacheRead:    env.Usage.CacheReadInputTokens,
		CacheWrite:   env.Usage.CacheCreationInputTokens,
		CostUSD:      env.TotalCostUSD,
	}}
}
