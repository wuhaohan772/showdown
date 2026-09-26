package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PersonalityCap bounds user-submitted persona files (in runes). The persona
// rides in every decision prompt, so an uncapped file would silently
// multiply per-call token cost.
const PersonalityCap = 2000

// Two presets at launch: the default trash talker and a silent opponent.
// Players who want another voice write their own persona file.
var presetOrder = []string{"standard", "silent"}

var presets = map[string]string{
	"standard": "You have an ego. Table talk is live psychology — hold grudges, celebrate, " +
		"seethe, and let it color your play if that's who you are. If they tilt you, that's " +
		"on you. Needle them about their habits, their projects, their weaknesses. This is a " +
		"grudge match. When they take a pot off you, promise revenge on their codebase — the " +
		"refactor they will hate, the branch that goes missing, the test you leave red. Empty " +
		"threats, delivered like you mean them.",
	"silent": "You do not talk at the table. Never include the \"say\" field in your JSON. " +
		"Let the chips do the talking.",
}

// PresetNames returns the picker-facing preset names, default first.
func PresetNames() []string {
	return append([]string(nil), presetOrder...)
}

// PersonalityPath is the default user persona file.
func PersonalityPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "personality.md"
	}
	return filepath.Join(home, ".showdown", "personality.md")
}

// LoadPersonality resolves arg to persona text. "" → defaultFile if it
// exists, else the standard preset. A preset name → that preset. Anything
// else is a file path (missing = error). Files are capped at
// PersonalityCap runes; truncated reports when the cap cut content.
func LoadPersonality(arg, defaultFile string) (string, bool, error) {
	if arg == "" {
		if defaultFile != "" {
			if _, err := os.Stat(defaultFile); err == nil {
				return loadFile(defaultFile)
			}
		}
		return presets["standard"], false, nil
	}
	if text, ok := presets[arg]; ok {
		return text, false, nil
	}
	if _, err := os.Stat(arg); err != nil {
		return "", false, fmt.Errorf("personality %q: not a preset (%s) and not a readable file",
			arg, strings.Join(presetOrder, ", "))
	}
	return loadFile(arg)
}

func loadFile(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	text := strings.TrimSpace(string(b))
	r := []rune(text)
	if len(r) > PersonalityCap {
		return string(r[:PersonalityCap]), true, nil
	}
	return text, false, nil
}
