package stats

import (
	"path/filepath"
	"testing"
)

func TestLoadMissingFileIsEmpty(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nope", "stats.json"))
	if err != nil || len(s) != 0 {
		t.Fatalf("got %v, %v — want empty, nil", s, err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "deep", "dir", "stats.json")
	s := Stats{"codex": {Wins: 3, Losses: 1}}
	if err := s.Save(p); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got["codex"] != (Record{Wins: 3, Losses: 1}) {
		t.Errorf("got %+v", got)
	}
}

func TestLine(t *testing.T) {
	s := Stats{"codex": {Wins: 3, Losses: 1}}
	if got := s.Line("codex"); got != "vs codex: 3–1" {
		t.Errorf("Line = %q", got)
	}
	if got := s.Line("claude"); got != "vs claude: 0–0" {
		t.Errorf("Line = %q", got)
	}
}

func TestRecordUsageRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "stats.json")
	s := Stats{"claude": {Wins: 1, TokensIn: 50000, TokensOut: 2000, CostUSD: 0.43}}
	if err := s.Save(p); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got["claude"] != s["claude"] {
		t.Errorf("got %+v", got["claude"])
	}
}

func TestLineIncludesCost(t *testing.T) {
	s := Stats{"claude": {Wins: 2, Losses: 1, CostUSD: 0.43}}
	if got := s.Line("claude"); got != "vs claude: 2–1 · $0.43" {
		t.Errorf("Line = %q", got)
	}
	// zero-cost records keep the old format exactly
	s2 := Stats{"codex": {Wins: 3, Losses: 1}}
	if got := s2.Line("codex"); got != "vs codex: 3–1" {
		t.Errorf("Line = %q", got)
	}
}
