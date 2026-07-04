# showdown

Heads-up No-Limit Texas Hold'em in your terminal — against whatever
coding-agent CLI you have installed. You use Codex? Codex plays you.
You use Claude? Same thing. It loads its own memory, so it knows you.
And yes, it trash talks.

## Play

    go install github.com/haohanwu/showdown@latest
    showdown

Detects `claude`, `codex`, and `gemini` on your PATH. Multiple found →
you choose your opponent. Force one with `--agent codex`. Pick the
model with `--model haiku` (cheaper and faster than the default). Match
cost shows on the match-over screen and accumulates in your career
stats. Silence the table talk with `--quiet`. Debugging something? `--debug` (or
`SHOWDOWN_DEBUG=1`) writes a full JSONL transcript — prompts, replies,
every action — to `~/.showdown/`, path printed on exit.

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
(see docs/adr/0002).
