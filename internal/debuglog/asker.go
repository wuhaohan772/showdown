package debuglog

import (
	"context"
	"time"

	"github.com/haohanwu/showdown/internal/agent"
)

// WrapAsker decorates ask so every agent CLI call is logged (event
// "agent_call") with the full prompt, raw output, error, and duration.
// Results pass through unchanged. A nil logger returns ask as-is.
func WrapAsker(ask agent.Asker, l *Logger) agent.Asker {
	if l == nil {
		return ask
	}
	return func(ctx context.Context, prompt string) (agent.Response, error) {
		start := time.Now()
		resp, err := ask(ctx, prompt)
		f := map[string]any{
			"prompt":      prompt,
			"raw":         resp.Text,
			"duration_ms": time.Since(start).Milliseconds(),
		}
		if err != nil {
			f["error"] = err.Error()
		}
		if resp.Usage != nil {
			u := resp.Usage
			f["tokens_in"] = u.InputTokens
			f["tokens_out"] = u.OutputTokens
			f["cache_read"] = u.CacheRead
			f["cache_write"] = u.CacheWrite
			f["cost_usd"] = u.CostUSD
		}
		l.Log("agent_call", f)
		return resp, err
	}
}
