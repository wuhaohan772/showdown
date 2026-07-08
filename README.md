# SHOWDOWN

Tired of waiting 20 minutes for your agent to completely miss your point? Mad that your agent burns through your 5-hour limit just to scan the code base? Can't believe that the newest models can't make you a million bucks with no mistakes?

It's time to settle this with a **SHOWDOWN!!!**

Heads-up No-Limit Texas Hold'em in your terminal — against whatever
coding-agent CLI you have installed. Claude Code, Codex, Cursor, Pi, if you have it installed, you can play against it. Agent loads its own memory, so it knows you. And yes, it trash talks.

## Install & play

    go install github.com/haohanwu/showdown@latest
    showdown

Showdown scans your `PATH` for `claude`, `codex`, and
`gemini`. Found more than one? It asks which one sits across the felt
from you. Found none? Install one first — showdown needs an agent CLI
to play against.

## First hand, walked through

1. Launch `showdown`. The menu lets you customize who you play against and how the game plays out. 
2. You're dealt two hole cards. Blinds are posted automatically (10/20
   to start).
3. When it's your turn: `f` fold, `c` check/call, `r` raise (then type
   an amount), `a` all-in. 
4. Watch the table talk — the agent reacts to your bets and its own
   hand, in character. Hit `t` any time to trash talk back.
5. Hand over → `enter` deals the next one. Blinds rise every 10 hands.
   Keep playing until someone busts.
6. Match over → cost and result post to your career stats.

## Make it yours

Force a specific opponent: `--agent codex`.

Pick the model: `--model haiku` is cheaper than the sonnet default,
but its table talk garbles who-folded-what — sonnet keeps the trash
talk honest.

Give the table a personality: `--personality unhinged` (also `needler`,
`polite`, `silent`, `degen`), or write your own at
`~/.showdown/personality.md` (capped at 2,000 chars).

Too much chatter? `--quiet` silences table talk entirely.

## The match

Sit-and-go: 1,500 chips each, blinds start 10/20 and rise every 10
hands, play until someone busts. Career record lives in
`~/.showdown/stats.json`.

## Keys

`f` fold · `c` check/call · `r` raise (then amount) · `a` all-in ·
`t` trash talk · `enter` next hand · `q` quit

## How it works

Each agent decision is one stateless CLI invocation: the game sends the
rules, the hand state, a digest of the match so far, and the table-talk
log; the agent replies with strict JSON. Garbage reply → one retry →
forced check/fold. The agent CLI runs in your launch directory so it
loads its own memory files — the trash talk is personal on purpose
(see docs/adr/0002). Claude opponents run as one persistent session per
match (prompt-cache warm: ~10x cheaper and faster decisions than
one-shot spawns); codex/gemini spawn per decision.

## Costs

Match cost shows on the match-over screen and accumulates in your
career stats. Career cost counts completed matches only; the `--debug`
transcript records every call's cost either way. Debugging something?
`--debug` (or `SHOWDOWN_DEBUG=1`) writes a full JSONL transcript —
prompts, replies, every action — to `~/.showdown/`, path printed on
exit.
