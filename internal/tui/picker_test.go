package tui

import (
	"strings"
	"testing"

	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/stats"
)

func pickerRoster() []agent.Adapter {
	return []agent.Adapter{{Key: "claude", DisplayName: "Claude Code", Bin: "claude",
		Args:   func(m, p string) []string { return nil },
		Models: []string{"haiku", "sonnet", "opus"}}}
}

func TestPickerModelSelection(t *testing.T) {
	a, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("2\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Model != "sonnet" {
		t.Errorf("Model = %q, want sonnet", a.Model)
	}
}

func TestPickerModelDefaultOnEmpty(t *testing.T) {
	a, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Model != "" {
		t.Errorf("Model = %q, want CLI default (empty)", a.Model)
	}
}

func TestPickerPresetModelSkipsPrompt(t *testing.T) {
	// no input available at all — preset must short-circuit before any Scan
	a, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader(""), "opus")
	if err != nil {
		t.Fatal(err)
	}
	if a.Model != "opus" {
		t.Errorf("Model = %q, want opus", a.Model)
	}
}
