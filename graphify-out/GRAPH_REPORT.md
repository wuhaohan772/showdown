# Graph Report - .  (2026-07-03)

## Corpus Check
- 12 files · ~34,261 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 309 nodes · 649 edges · 18 communities (13 shown, 5 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 78 edges (avg confidence: 0.8)
- Token cost: 66,048 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_TUI App Core (Model & View)|TUI App Core (Model & View)]]
- [[_COMMUNITY_Card & Deck|Card & Deck]]
- [[_COMMUNITY_Debug Logger (debuglog)|Debug Logger (debuglog)]]
- [[_COMMUNITY_TUI Flow & Debug-Log Tests|TUI Flow & Debug-Log Tests]]
- [[_COMMUNITY_Agent Adapter & Roster|Agent Adapter & Roster]]
- [[_COMMUNITY_Betting Engine All-In Tests|Betting Engine All-In Tests]]
- [[_COMMUNITY_Decision Extraction & Fallback|Decision Extraction & Fallback]]
- [[_COMMUNITY_Betting Engine Core (Hand.Apply)|Betting Engine Core (Hand.Apply)]]
- [[_COMMUNITY_Digest & Prompt Rendering|Digest & Prompt Rendering]]
- [[_COMMUNITY_Debug & Talk Feature Specs|Debug & Talk Feature Specs]]
- [[_COMMUNITY_Domain Glossary (CONTEXT.md)|Domain Glossary (CONTEXT.md)]]
- [[_COMMUNITY_Match & Blind Schedule|Match & Blind Schedule]]
- [[_COMMUNITY_Prompt Spike Harness|Prompt Spike Harness]]
- [[_COMMUNITY_Stub Agent (adapter test)|Stub Agent (adapter test)]]
- [[_COMMUNITY_Stub Agent (e2e)|Stub Agent (e2e)]]
- [[_COMMUNITY_Roster (isolated)|Roster (isolated)]]
- [[_COMMUNITY_Rand (isolated)|Rand (isolated)]]
- [[_COMMUNITY_Go Module Root|Go Module Root]]

## God Nodes (most connected - your core abstractions)
1. `Model` - 27 edges
2. `Hand` - 25 edges
3. `NewHand()` - 18 edges
4. `testModel()` - 17 edges
5. `key()` - 17 edges
6. `Card` - 15 edges
7. `GetDecision()` - 13 edges
8. `mustApply()` - 13 edges
9. `NewModel()` - 13 edges
10. `newTestHand()` - 12 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `NewModel()`  [INFERRED]
  main.go → internal/tui/app.go
- `main()` --calls--> `DefaultPath()`  [INFERRED]
  main.go → internal/debuglog/debuglog.go
- `main()` --calls--> `New()`  [INFERRED]
  main.go → internal/debuglog/debuglog.go
- `Showdown` --references--> `Debug Transcript Log Design`  [EXTRACTED]
  README.md → docs/superpowers/specs/2026-07-03-debug-transcript-log-design.md
- `GetDecision()` --calls--> `RenderPrompt()`  [INFERRED]
  internal/agent/decide.go → internal/agent/prompt.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Debug Event Capture Pipeline** — docs_superpowers_specs_2026_07_03_debug_transcript_log_design_logger, docs_superpowers_specs_2026_07_03_debug_transcript_log_design_wrapasker, docs_superpowers_specs_2026_07_03_debug_transcript_log_design_apply_helper [EXTRACTED 1.00]
- **Phase-Aware Talk Input Management** — docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_talkreturn_field, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_opentalk_helper, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_phase_widening [EXTRACTED 1.00]
- **Async State Preservation Pattern** — docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_talkreturn_field, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_decision_race_handling, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_runout_tick_handling [EXTRACTED 1.00]
- **Decision Pipeline: Request → Decision → Action** — context_md_decision_request, context_md_decision, context_md_fallback_action [INFERRED 0.85]
- **ADR Implementation in Code** — docs_adr_0001_stateless_decision_requests, docs_adr_0002_native_memory_loading, superpowers_sdd_task_9_report_get_decision, superpowers_sdd_task_10_brief_asker_method [INFERRED 0.75]

## Communities (18 total, 5 thin omitted)

### Community 0 - "TUI App Core (Model & View)"
Cohesion: 0.11
Nodes (25): Action, ActionType, Card, Cmd, Digest, Hand, cardStrings(), Adapter (+17 more)

### Community 1 - "Card & Deck"
Cohesion: 0.12
Nodes (30): Rand, NewDeck(), ParseCard(), T, TestCardString(), TestDeckDeals52Unique(), TestDeckShuffled(), TestParseCardRoundTrip() (+22 more)

### Community 2 - "Debug Logger (debuglog)"
Cohesion: 0.12
Nodes (22): Asker, Logger, File, T, TestWrapAskerLogsError(), TestWrapAskerNilLogger(), TestWrapAskerPassthroughAndLog(), WrapAsker() (+14 more)

### Community 3 - "TUI Flow & Debug-Log Tests"
Cohesion: 0.24
Nodes (27): NewModel(), KeyMsg, T, key(), readLogEvents(), TestAgentDecisionLogged(), TestAgentDecisionMsgApplies(), TestDebugLogCapturesHandFlow() (+19 more)

### Community 4 - "Agent Adapter & Roster"
Cohesion: 0.11
Nodes (16): Adapter, Duration, DetectRoster(), T, TestAskerTimeout(), TestCustomAdapterViaEnv(), TestKnownAdapterArgs(), TestWhitespaceCustomCmdFallsBack() (+8 more)

### Community 5 - "Betting Engine All-In Tests"
Cohesion: 0.26
Nodes (22): Rand, NewHand(), T, hasAction(), mustApply(), newTestHand(), TestAllInActorHasNoActions(), TestAllInCallRunsOutBoard() (+14 more)

### Community 6 - "Decision Extraction & Fallback"
Cohesion: 0.23
Nodes (20): Asker, Decision, Context, ExtractDecision(), FallbackAction(), GetDecision(), GetReaction(), ReactionPrompt() (+12 more)

### Community 7 - "Betting Engine Core (Hand.Apply)"
Cohesion: 0.19
Nodes (5): Hand, LogItem, Result, SeatState, Street

### Community 8 - "Digest & Prompt Rendering"
Cohesion: 0.16
Nodes (10): Digest, RequestData, NewDigest(), BuildHandState(), RenderPrompt(), T, TestBuildHandState(), TestBuildHandStatePerspectiveIndependentOfActor() (+2 more)

### Community 9 - "Debug & Talk Feature Specs"
Cohesion: 0.16
Nodes (18): Debug Transcript Log Implementation Plan, Trash-Talk Echo & Phase Widening Implementation Plan, apply Helper, Bug Report Protocol, Debug Transcript Log Design, internal/debuglog Package, JSONL Format, Logger (+10 more)

### Community 10 - "Domain Glossary (CONTEXT.md)"
Cohesion: 0.15
Nodes (17): Agent Adapter, Career Record, Decision, Decision Request, Fallback Action, Game, Hand-End Reaction, Match (+9 more)

### Community 11 - "Match & Blind Schedule"
Cohesion: 0.22
Nodes (6): NewMatch(), T, TestBlindSchedule(), TestButtonAlternates(), TestNextHandAndBust(), Match

### Community 12 - "Prompt Spike Harness"
Cohesion: 0.53
Nodes (5): build_prompt(), extract_json(), main(), Find the last parseable JSON object with an 'action' key., run_one()

## Knowledge Gaps
- **16 isolated node(s):** `github.com/haohanwu/showdown`, `stub.sh script`, `Record`, `stub-agent.sh script`, `Game` (+11 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **5 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Hand` connect `Betting Engine Core (Hand.Apply)` to `Digest & Prompt Rendering`, `Card & Deck`, `Betting Engine All-In Tests`?**
  _High betweenness centrality (0.090) - this node is a cross-community bridge._
- **Why does `GetDecision()` connect `Decision Extraction & Fallback` to `Digest & Prompt Rendering`?**
  _High betweenness centrality (0.083) - this node is a cross-community bridge._
- **Why does `Asker` connect `Decision Extraction & Fallback` to `Agent Adapter & Roster`?**
  _High betweenness centrality (0.065) - this node is a cross-community bridge._
- **Are the 12 inferred relationships involving `NewHand()` (e.g. with `TestBuildHandState()` and `TestBuildHandStatePerspectiveIndependentOfActor()`) actually correct?**
  _`NewHand()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/haohanwu/showdown`, `stub.sh script`, `Record` to the rest of the system?**
  _18 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `TUI App Core (Model & View)` be split into smaller, more focused modules?**
  _Cohesion score 0.10853658536585366 - nodes in this community are weakly interconnected._
- **Should `Card & Deck` be split into smaller, more focused modules?**
  _Cohesion score 0.11522048364153627 - nodes in this community are weakly interconnected._