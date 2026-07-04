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
	a, _, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("2\n"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Model != "sonnet" {
		t.Errorf("Model = %q, want sonnet", a.Model)
	}
}

func TestPickerModelDefaultOnEmpty(t *testing.T) {
	a, persona, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("\n\n"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Model != "" {
		t.Errorf("Model = %q, want CLI default (empty)", a.Model)
	}
	if persona != "" {
		t.Errorf("persona = %q, want empty (default resolution in main)", persona)
	}
}

func TestPickerPresetModelSkipsPrompt(t *testing.T) {
	// preset skips the model prompt; the persona step may Scan and hit EOF, which falls back to default
	a, _, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader(""), "opus", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Model != "opus" {
		t.Errorf("Model = %q, want opus", a.Model)
	}
}

func TestPickerPersonalitySelection(t *testing.T) {
	// single claude-like roster: model step then persona step
	_, persona, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("\n3\n"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if persona != "polite" {
		t.Errorf("persona = %q, want polite (3rd preset)", persona)
	}
}

func TestPickerPersonalityDefaultOnEmpty(t *testing.T) {
	_, persona, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("\n\n"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if persona != "" {
		t.Errorf("persona = %q, want empty (default resolution in main)", persona)
	}
}

func TestPickerPresetPersonalitySkipsPrompt(t *testing.T) {
	_, persona, err := RunPicker(pickerRoster(), stats.Stats{}, strings.NewReader("\n"), "", "degen")
	if err != nil {
		t.Fatal(err)
	}
	if persona != "degen" {
		t.Errorf("persona = %q, want degen", persona)
	}
}
