# CONTEXT

Glossary of domain terms for this project. Definitions only — no implementation details.

## Glossary

**Game** — Heads-up (1v1) No-Limit Texas Hold'em between the human player and one Opponent Agent, played in the terminal with ASCII art presentation.

**Opponent Agent** — The AI opponent. Not built into the game: it is an external coding-agent CLI already installed on the user's machine (e.g. Claude Code, Codex, Gemini CLI). "Plug and go": whichever agent CLI the user has becomes the opponent.

**Agent Adapter** — The boundary between the game and a specific agent CLI. Knows how to ask that CLI for a poker decision and interpret its answer. One adapter per supported CLI.

**Match** — One sitting: heads-up sit-and-go. Both players start with equal fixed stacks, blinds escalate on a schedule, play continues until one player has all the chips. A Match has exactly one winner.

**Decision Request** — A single, self-contained ask sent to the Opponent Agent when it must act. Stateless: contains everything the agent needs (rules, hand history, current state, table talk). No conversation persists between requests.

**Decision** — The agent's answer to a Decision Request: one poker action (fold / check / call / bet / raise with amount) plus optional Table Talk.

**Fallback Action** — The forced action substituted when the agent's reply is unusable after one retry: check if checking is legal, otherwise fold. Guarantees a Match can never stall on a misbehaving agent.

**Table Talk** — In-game speech. The agent may attach a one-liner to each Decision; the player may type trash talk back, which is included in subsequent Decision Requests. Cosmetic to the rules, psychological to the game.

**Roster** — The set of agent CLIs detected on the user's machine, from which the opponent is chosen.

**Match Digest** — Compact summary of the match so far (stack trajectory, prior hand results, revealed showdowns, Table Talk log) included in every Decision Request. How the agent "remembers" and develops reads despite stateless requests.

**Career Record** — Persistent win/loss tally of Matches, kept per agent (e.g. "vs codex: 3–1"). Survives between sessions.

**Difficulty** — How hard the Opponent Agent thinks. *Instinct* (default): no tool execution — answers from the Decision Request plus its Native Memory. *Cooking*: may use its tools (run code, compute equity) before deciding.

**Native Memory** — The persistent context an agent CLI loads about its user on its own (e.g. CLAUDE.md, AGENTS.md, memory directories). The game never reads these files; the agent brings them to the table itself. Source of personal Table Talk.

**Tilt** — Table Talk is real psychology, not decoration: the agent is free to let taunts, grudges, and bad beats influence its Decisions. Tilting your opponent is a legitimate strategy, in both directions.

**Hand-End Reaction** — A speech-only response from the agent after a hand concludes (gloat, whine, needle). Happens in the background; never delays the next hand.
