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
