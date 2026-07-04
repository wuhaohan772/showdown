# Menu Screen — Design

Date: 2026-07-04
Status: approved

## Purpose

P4 of the roadmap: replace the three sequential stdin prompts of
`tui.RunPicker` (opponent → model → persona) with a single-screen
bubbletea dashboard that packages opponent select, model select,
persona select, table-talk toggle, and career stats in one view.
Fewest keystrokes to a match: arrow to what you want, enter to deal.

```
♠ SHOWDOWN

  opponent     ◀ Claude Code ▶      vs claude: 12–8 · $1.42
  model        ◀ haiku ▶
  persona      ◀ needler ▶
  table talk   ◀ on ▶

  [ enter ] deal me in    [ q ] quit
```

## Architecture

Separate bubbletea program, inline (no alt-screen), run before the
game program starts. New file `internal/tui/menu.go`:

```go
func RunMenu(roster []agent.Adapter, st stats.Stats,
    presetModel, presetPersonality string, presetQuiet bool,
) (agent.Adapter, string, bool, error)
```

Returns the chosen adapter (with `.Model` set), the personality
argument (same semantics as today: preset name or `""` = default —
resolution to text stays in main via `agent.LoadPersonality`), and
the quiet bool. `main.go` swaps the `RunPicker` call for `RunMenu`
and threads the returned quiet into `NewModel` instead of the raw
flag. The `--agent` fast path bypasses the menu entirely, unchanged.

`RunPicker` and `picker_test.go` are deleted.

## Rows

1. **opponent** — cycles `roster`. Right side of the row shows
   `st.Line(key)` for the currently selected opponent (the stats
   requirement lives here; no separate stats view).
2. **model** — cycles the selected adapter's `Models` with a
   `(default)` entry first (maps to `""`). Adapter with an empty
   `Models` list renders a fixed `(default)`; arrows no-op.
   Changing opponent resets this row to that adapter's initial entry
   (`(default)`, or the `--model` prefill when the flag was given).
3. **persona** — cycles `agent.PresetNames()`. If
   `agent.PersonalityPath()` exists, a `custom` entry (maps to `""`,
   i.e. LoadPersonality's file-first default resolution) is prepended
   and initially selected.
4. **table talk** — `on`/`off`; `off` returns quiet = true.

## Flags

`--model`, `--personality`, `--quiet` pre-fill their rows instead of
skipping prompts (a dashboard makes skipping pointless). Pre-fill
matching: `--model` selects the matching Models entry, or is kept
verbatim as an extra cycle entry if not in the list (flag-only models
stay usable); `--personality` selects the matching preset name, or is
kept verbatim as an extra entry (covers file paths). Single-adapter
roster still shows the menu (row just doesn't cycle).

## Keys

- `↑`/`↓` (and `k`/`j`): move row focus
- `←`/`→` (and `h`/`l`): cycle focused row's value
- `enter`: start match with current selections
- `q` / `ctrl+c`: quit — clean exit (main exits 0, nothing printed)

Quit signalling: `RunMenu` returns a sentinel `ErrMenuQuit`; main
treats it as a silent `os.Exit(0)`.

## Errors

Empty roster → same error string as today ("no agent CLIs found on
PATH…"), returned before any program starts.

## Rendering

Lipgloss-light, matching the game's aesthetic: `♠ SHOWDOWN` header,
focused row marked (arrow markers `◀ ▶` on the focused row's value,
plain value on unfocused rows), dim key-hint line at the bottom.

## Testing

Unit-test `menuModel` `Update`/`View` directly, same pattern as
`app_test.go` — no teatest dependency:

- cycling each row wraps correctly; model row resets on opponent change
- adapter without Models: model row fixed at `(default)`
- persona list gains `custom` first when the personality file exists
  (path injected for the test)
- pre-fill: flags select matching entries; unknown values appended
- enter returns adapter with model set, persona arg, quiet bool
- q returns ErrMenuQuit
- View contains the stats line for the selected opponent
