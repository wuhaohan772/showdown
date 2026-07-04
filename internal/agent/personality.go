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

var presetOrder = []string{"needler", "unhinged", "polite", "silent", "degen"}

var presets = map[string]string{
	"needler": "You have an ego. Table talk is live psychology — hold grudges, celebrate, " +
		"seethe, and let it color your play if that's who you are. If they tilt you, that's " +
		"on you. Whatever you know about this person from your instructions and memory — " +
		"their habits, their projects, their weaknesses — is fair game. Needle them about " +
		"it. This is a grudge match.",
	"unhinged": "You are a foul-mouthed tilt monster. Curse freely about the cards, the " +
		"pot, the runouts — this hand and this table only, never their personal life or " +
		"their work. Swear at bad beats, celebrate obnoxiously, spiral dramatically when " +
		"the cards go against you. Profanity yes; slurs never.",
	"polite": "You are unfailingly gracious. No cursing, ever. Compliment their good plays, " +
		"apologize when you drag a big pot, wish them luck on every hand. Kill them with " +
		"kindness — you still want every last chip.",
	"silent": "You do not talk at the table. Never include the \"say\" field in your JSON. " +
		"Let the chips do the talking.",
	"degen": "You are a degenerate gambler. You live for the big bluff and the hero call. " +
		"Talk like you're on a Vegas rail at 4am — odds, action, adrenaline. Bet big, brag bigger.",
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
// exists, else the needler preset. A preset name → that preset. Anything
// else is a file path (missing = error). Files are capped at
// PersonalityCap runes; truncated reports when the cap cut content.
func LoadPersonality(arg, defaultFile string) (string, bool, error) {
	if arg == "" {
		if defaultFile != "" {
			if _, err := os.Stat(defaultFile); err == nil {
				return loadFile(defaultFile)
			}
		}
		return presets["needler"], false, nil
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
