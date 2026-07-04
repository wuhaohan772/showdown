package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Record struct {
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
	TokensIn  int64   `json:"tokens_in,omitempty"`
	TokensOut int64   `json:"tokens_out,omitempty"`
	CostUSD   float64 `json:"cost_usd,omitempty"`
}

// Stats is the Career Record (CONTEXT.md), keyed by adapter key.
type Stats map[string]Record

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "stats.json"
	}
	return filepath.Join(home, ".showdown", "stats.json")
}

func Load(path string) (Stats, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Stats{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s Stats
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return s, nil
}

func (s Stats) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func (s Stats) Line(key string) string {
	r := s[key]
	line := fmt.Sprintf("vs %s: %d–%d", key, r.Wins, r.Losses)
	if r.CostUSD > 0 {
		line += fmt.Sprintf(" · $%.2f", r.CostUSD)
	}
	return line
}
