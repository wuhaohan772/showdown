// internal/agent/response_test.go
package agent

import "testing"

const sampleEnvelope = `{"type":"result","subtype":"success","is_error":false,
"duration_ms":8951,"result":"{\"action\": \"call\"}","total_cost_usd":0.021552,
"usage":{"input_tokens":10,"cache_creation_input_tokens":9039,
"cache_read_input_tokens":10690,"output_tokens":479}}`

func TestParseClaudeJSON(t *testing.T) {
	r := ParseClaudeJSON(sampleEnvelope)
	if r.Text != `{"action": "call"}` {
		t.Errorf("Text = %q", r.Text)
	}
	if r.Usage == nil {
		t.Fatal("Usage = nil, want populated")
	}
	u := *r.Usage
	if u.InputTokens != 10 || u.OutputTokens != 479 || u.CacheRead != 10690 || u.CacheWrite != 9039 {
		t.Errorf("Usage = %+v", u)
	}
	if u.CostUSD != 0.021552 {
		t.Errorf("CostUSD = %v", u.CostUSD)
	}
	if u.TotalIn() != 10+10690+9039 {
		t.Errorf("TotalIn = %d", u.TotalIn())
	}
}

func TestParseClaudeJSONMalformedFallsBackToRaw(t *testing.T) {
	for _, raw := range []string{"plain text reply", `{"no_result_key":1}`, ""} {
		r := ParseClaudeJSON(raw)
		if r.Text != raw {
			t.Errorf("ParseClaudeJSON(%q).Text = %q, want raw passthrough", raw, r.Text)
		}
		if r.Usage != nil {
			t.Errorf("ParseClaudeJSON(%q).Usage != nil", raw)
		}
	}
}

func TestUsageAdd(t *testing.T) {
	u := Usage{InputTokens: 1, OutputTokens: 2, CacheRead: 3, CacheWrite: 4, CostUSD: 0.5}
	u.Add(&Usage{InputTokens: 10, OutputTokens: 20, CacheRead: 30, CacheWrite: 40, CostUSD: 0.25})
	want := Usage{InputTokens: 11, OutputTokens: 22, CacheRead: 33, CacheWrite: 44, CostUSD: 0.75}
	if u != want {
		t.Errorf("got %+v, want %+v", u, want)
	}
	u.Add(nil) // must be a no-op, not a panic
	if u != want {
		t.Errorf("Add(nil) changed value: %+v", u)
	}
}
