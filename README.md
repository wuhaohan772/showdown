# showdown

Heads-up No-Limit Texas Hold'em in your terminal, against whatever
coding-agent CLI you have installed: Claude Code, Codex, or Gemini.
The agent runs from your launch directory, so it loads its own memory
files and knows who it's playing. It trash talks accordingly.

## Install

    go install github.com/haohanwu/showdown@latest
    showdown

Showdown scans your `PATH` for `claude`, `codex`, and `gemini`. If it
finds more than one, the menu asks which one you want across the felt.
If it finds none, install an agent CLI first; showdown has no built-in
opponent.

## Playing your first match

1. Run `showdown`. The menu shows your opponent, model, persona, and
   table-talk setting. Adjust anything, then start.
2. You get two hole cards. Blinds post automatically, starting at
   10/20.
3. On your turn: `f` fold, `c` check/call, `r` raise (then type an
   amount), `a` all-in.
4. The agent talks while it plays. Its chat runs in the panel on the
   right (or in the log line on narrow terminals). Press `t` any time
   to talk back.
5. When a hand ends, `enter` deals the next one. Blinds rise every 10
   hands. Play continues until someone busts.
6. At match over, the agent gets one parting shot, then the result and
   API cost post to your career stats.

## Keys

`f` fold · `c` check/call · `r` raise (then amount) · `a` all-in ·
`t` trash talk · `enter` next hand · `q` quit

## Options

| Flag | What it does |
|------|--------------|
| `--agent claude` | Skip the picker, force this opponent (`claude`, `codex`, `gemini`) |
| `--model sonnet` | Model for the agent CLI. Claude: `haiku`/`sonnet`/`opus`; codex/gemini values pass through |
| `--personality unhinged` | Preset name or path to your own `.md` file (see below) |
| `--stack 1500` | Starting chips per player |
| `--blind 10` | Starting small blind (big blind is always 2x) |
| `--hands 20` | Cap the match at N hands (0 = play until bust) |
| `--quiet` | No table talk |
| `--debug` | Write a full JSONL transcript to `~/.showdown/` |

## Personas

Presets: `needler` (default), `unhinged`, `polite`, `silent`, `degen`.

To write your own, put a `personality.md` at `~/.showdown/` (used
automatically) or pass any path with `--personality path/to/file.md`.
Cap is 2,000 characters. Describe how the opponent should talk; the
game handles the poker.

One thing showdown deliberately ignores: your Claude Code hooks and
plugins. They inject into every claude session, so a style plugin on
your machine (say, one that compresses all output into caveman speak)
would warp the opponent's table talk too. Showdown disables hooks in
the opponent's session; your memory files still load. If you want the
opponent to talk differently, use a persona — that's what it's for.

## Choosing a model (Claude)

`sonnet` is the default and the recommendation. It plays fine and its
trash talk stays factually straight about who folded what.

`haiku` is cheaper and works, with one known quirk: right after
folding, it blames *you* for the fold about half the time ("can't
believe you folded that"). This is a model limitation, not a bug we
can prompt around, so if the taunts stop making sense on haiku, that's
why. Switch to sonnet if it bothers you.

## Costs

Every agent decision is a real API call through your agent CLI, billed
however that CLI bills you. The match-over screen shows what the match
cost, and completed matches accumulate into career stats at
`~/.showdown/stats.json`. Claude opponents keep one persistent session
per match, so decisions after the first are prompt-cache warm (roughly
10x cheaper than one-shot calls). A typical sonnet match runs well
under a dollar.

Quitting mid-match skips the career-cost entry; the `--debug`
transcript records every call's cost either way.

## When something looks wrong

Run with `--debug` (or `SHOWDOWN_DEBUG=1`). It writes a JSONL
transcript of every prompt, reply, and action to `~/.showdown/`, and
prints the path on exit. That file is the first thing to attach to a
bug report.

## How it works

The game sends the agent the rules, the current hand state, a digest of
the match so far, and the table-talk log; the agent replies with strict
JSON. A garbage reply gets one retry, then a forced check/fold. The
agent CLI runs in your launch directory on purpose, so it can read its
own memory and make the needling personal (see docs/adr/0002). Claude
runs as one long-lived session per match; codex and gemini spawn one
process per decision.
