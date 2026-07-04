package tui

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/stats"
)

// RunPicker chooses the opponent, optionally its model, and optionally a
// table persona, before the TUI starts. presetModel / presetPersonality
// (from --model / --personality) skip their prompts. The returned string
// is the personality argument (preset name, path, or "" = default) —
// resolution to text happens in main via agent.LoadPersonality.
func RunPicker(roster []agent.Adapter, st stats.Stats, in io.Reader, presetModel, presetPersonality string) (agent.Adapter, string, error) {
	if len(roster) == 0 {
		return agent.Adapter{}, "", fmt.Errorf("no agent CLIs found on PATH (looked for: claude, codex, gemini)")
	}
	sc := bufio.NewScanner(in)
	chosen := roster[0]
	if len(roster) > 1 {
		fmt.Println("♠ choose your opponent:")
		for i, a := range roster {
			fmt.Printf("  %d. %-12s %s\n", i+1, a.DisplayName, st.Line(a.Key))
		}
		fmt.Print("> ")
		if !sc.Scan() {
			return agent.Adapter{}, "", fmt.Errorf("no selection")
		}
		n, err := strconv.Atoi(strings.TrimSpace(sc.Text()))
		if err != nil || n < 1 || n > len(roster) {
			return agent.Adapter{}, "", fmt.Errorf("pick a number 1-%d", len(roster))
		}
		chosen = roster[n-1]
	}
	if presetModel != "" {
		chosen.Model = presetModel
	} else if len(chosen.Models) > 0 {
		fmt.Printf("♦ model for %s (enter = default):\n", chosen.DisplayName)
		for i, mo := range chosen.Models {
			fmt.Printf("  %d. %s\n", i+1, mo)
		}
		fmt.Print("> ")
		if sc.Scan() {
			txt := strings.TrimSpace(sc.Text())
			if txt != "" {
				n, err := strconv.Atoi(txt)
				if err != nil || n < 1 || n > len(chosen.Models) {
					return agent.Adapter{}, "", fmt.Errorf("pick a number 1-%d or enter for default", len(chosen.Models))
				}
				chosen.Model = chosen.Models[n-1]
			}
		}
	}
	if presetPersonality != "" {
		return chosen, presetPersonality, nil
	}
	names := agent.PresetNames()
	fmt.Println("♣ table persona (enter = default):")
	for i, n := range names {
		fmt.Printf("  %d. %s\n", i+1, n)
	}
	fmt.Print("> ")
	if sc.Scan() {
		txt := strings.TrimSpace(sc.Text())
		if txt != "" {
			n, err := strconv.Atoi(txt)
			if err != nil || n < 1 || n > len(names) {
				return agent.Adapter{}, "", fmt.Errorf("pick a number 1-%d or enter for default", len(names))
			}
			return chosen, names[n-1], nil
		}
	}
	return chosen, "", nil
}
