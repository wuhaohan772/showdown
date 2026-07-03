# Graph Report - .  (2026-07-02)

## Corpus Check
- Corpus is ~44,257 words - fits in a single context window. You may not need a graph.

## Summary
- 282 nodes · 577 edges · 22 communities (13 shown, 9 thin omitted)
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 73 edges (avg confidence: 0.8)
- Token cost: 137,748 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Poker Engine Design Rationale|Poker Engine Design Rationale]]
- [[_COMMUNITY_Agent Adapter & Roster Detection|Agent Adapter & Roster Detection]]
- [[_COMMUNITY_TUI Bubble Tea State Machine|TUI Bubble Tea State Machine]]
- [[_COMMUNITY_Domain Glossary (CONTEXT.md)|Domain Glossary (CONTEXT.md)]]
- [[_COMMUNITY_Card & Hand Evaluation|Card & Hand Evaluation]]
- [[_COMMUNITY_Betting Engine All-In Tests|Betting Engine All-In Tests]]
- [[_COMMUNITY_Decision Extraction & Fallback|Decision Extraction & Fallback]]
- [[_COMMUNITY_Prompt Rendering & Match Digest|Prompt Rendering & Match Digest]]
- [[_COMMUNITY_TUI Flow Tests|TUI Flow Tests]]
- [[_COMMUNITY_Betting Engine Core (Hand.Apply)|Betting Engine Core (Hand.Apply)]]
- [[_COMMUNITY_Match Structure & Blind Schedule|Match Structure & Blind Schedule]]
- [[_COMMUNITY_ASCII Card Rendering|ASCII Card Rendering]]
- [[_COMMUNITY_Prompt Spike Harness|Prompt Spike Harness]]
- [[_COMMUNITY_Stub Agent Script (adapter test)|Stub Agent Script (adapter test)]]
- [[_COMMUNITY_Stub Agent Script (e2e)|Stub Agent Script (e2e)]]
- [[_COMMUNITY_Task 9 Report Notes|Task 9 Report Notes]]
- [[_COMMUNITY_Bubbletea Dependency|Bubbletea Dependency]]
- [[_COMMUNITY_Go Module Root|Go Module Root]]
- [[_COMMUNITY_Stats Record Struct (Brief)|Stats Record Struct (Brief)]]
- [[_COMMUNITY_Render Card (Brief Note)|Render Card (Brief Note)]]
- [[_COMMUNITY_Fallback Action (Report Note)|Fallback Action (Report Note)]]

## God Nodes (most connected - your core abstractions)
1. `Hand` - 26 edges
2. `Model` - 23 edges
3. `NewHand()` - 19 edges
4. `Card` - 15 edges
5. `GetDecision()` - 14 edges
6. `mustApply()` - 13 edges
7. `newTestHand()` - 12 edges
8. `Match` - 10 edges
9. `testModel()` - 10 edges
10. `showdown-v1 Branch Project` - 10 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `DefaultPath()`  [INFERRED]
  main.go → internal/stats/stats.go
- `main()` --calls--> `DetectRoster()`  [INFERRED]
  main.go → internal/agent/adapter.go
- `main()` --calls--> `Load()`  [INFERRED]
  main.go → internal/stats/stats.go
- `main()` --calls--> `NewModel()`  [INFERRED]
  main.go → internal/tui/app.go
- `main()` --calls--> `RunPicker()`  [INFERRED]
  main.go → internal/tui/picker.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Core Poker Engine Components** — internal_poker_card, internal_poker_deck, internal_poker_hand, internal_poker_street_types, internal_poker_action_type, internal_poker_seat_state, internal_poker_result [EXTRACTED 1.00]
- **Match and Tournament System** — internal_poker_match, blind_schedule_mechanism, button_rotation_heads_up [EXTRACTED 1.00]
- **Agent Decision and Communication Pipeline** — internal_agent_prompt, internal_agent_digest, internal_agent_decision, fallback_strategy_adr_0001 [EXTRACTED 1.00]
- **Decision Pipeline: Request → Decision → Action** — context_md_decision_request, context_md_decision, context_md_fallback_action [INFERRED 0.85]
- **Agent Integration Tasks (9, 10, 14)** — superpowers_sdd_task_9_report_get_decision, superpowers_sdd_task_10_brief_adapter_struct, superpowers_sdd_task_14_brief_picker [INFERRED 0.85]
- **ADR Implementation in Code** — docs_adr_0001_stateless_decision_requests, docs_adr_0002_native_memory_loading, superpowers_sdd_task_9_report_get_decision, superpowers_sdd_task_10_brief_asker_method [INFERRED 0.75]

## Communities (22 total, 9 thin omitted)

### Community 0 - "Poker Engine Design Rationale"
Cohesion: 0.08
Nodes (29): All-In Edge Case Coverage - Blind Post Short Stacks, Short Calls, Escalating Blind Levels (10/20 to 200/400), Button Rotation in Heads-Up Play, chehsunliu/poker Hand Evaluator Library, Chip Conservation in All-In Edge Cases, Stateless Decision Fallback (ADR-0001), Heads-Up Poker (2-Player) Game Format, Decision Type and Extraction Logic (+21 more)

### Community 1 - "Agent Adapter & Roster Detection"
Cohesion: 0.11
Nodes (20): Adapter, Duration, DetectRoster(), T, TestAskerTimeout(), TestCustomAdapterViaEnv(), TestKnownAdapterArgs(), TestWhitespaceCustomCmdFallsBack() (+12 more)

### Community 2 - "TUI Bubble Tea State Machine"
Cohesion: 0.17
Nodes (14): Cmd, KeyMsg, Rand, indent(), joinActions(), runoutTick(), Msg, Result (+6 more)

### Community 3 - "Domain Glossary (CONTEXT.md)"
Cohesion: 0.11
Nodes (27): Agent Adapter, Career Record, Decision, Decision Request, Fallback Action, Game, Hand-End Reaction, Match (+19 more)

### Community 4 - "Card & Hand Evaluation"
Cohesion: 0.16
Nodes (20): Rand, NewDeck(), ParseCard(), T, TestCardString(), TestDeckDeals52Unique(), TestDeckShuffled(), TestParseCardRoundTrip() (+12 more)

### Community 5 - "Betting Engine All-In Tests"
Cohesion: 0.26
Nodes (22): Rand, NewHand(), T, hasAction(), mustApply(), newTestHand(), TestAllInActorHasNoActions(), TestAllInCallRunsOutBoard() (+14 more)

### Community 6 - "Decision Extraction & Fallback"
Cohesion: 0.23
Nodes (20): Asker, Decision, Context, ExtractDecision(), FallbackAction(), GetDecision(), GetReaction(), ReactionPrompt() (+12 more)

### Community 7 - "Prompt Rendering & Match Digest"
Cohesion: 0.14
Nodes (12): Digest, RequestData, NewDigest(), BuildHandState(), RenderPrompt(), T, TestBuildHandState(), TestBuildHandStatePerspectiveIndependentOfActor() (+4 more)

### Community 8 - "TUI Flow Tests"
Cohesion: 0.33
Nodes (14): handSummary(), KeyMsg, T, key(), TestAgentDecisionMsgApplies(), TestFallbackNoticeVisibleInQuietMode(), TestHandStartsWithHumanOrAgentTurn(), TestHandSummaryDecisiveWin() (+6 more)

### Community 10 - "Match Structure & Blind Schedule"
Cohesion: 0.23
Nodes (6): NewMatch(), T, TestBlindSchedule(), TestButtonAlternates(), TestNextHandAndBust(), Match

### Community 11 - "ASCII Card Rendering"
Cohesion: 0.36
Nodes (10): rankLabel(), RenderCard(), RenderCardRow(), SuitGlyph(), card(), T, stripANSI(), TestRenderCardFaceDown() (+2 more)

### Community 12 - "Prompt Spike Harness"
Cohesion: 0.53
Nodes (5): build_prompt(), extract_json(), main(), Find the last parseable JSON object with an 'action' key., run_one()

## Knowledge Gaps
- **21 isolated node(s):** `github.com/haohanwu/showdown`, `stub.sh script`, `Record`, `startHandMsg`, `runoutTickMsg` (+16 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **9 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Model` connect `TUI Bubble Tea State Machine` to `Agent Adapter & Roster Detection`, `Prompt Rendering & Match Digest`, `TUI Flow Tests`, `Betting Engine Core (Hand.Apply)`, `Match Structure & Blind Schedule`?**
  _High betweenness centrality (0.271) - this node is a cross-community bridge._
- **Why does `Hand` connect `Betting Engine Core (Hand.Apply)` to `Poker Engine Design Rationale`, `TUI Bubble Tea State Machine`, `Card & Hand Evaluation`, `Betting Engine All-In Tests`, `Prompt Rendering & Match Digest`?**
  _High betweenness centrality (0.232) - this node is a cross-community bridge._
- **Why does `Card` connect `Card & Hand Evaluation` to `Betting Engine Core (Hand.Apply)`, `ASCII Card Rendering`?**
  _High betweenness centrality (0.098) - this node is a cross-community bridge._
- **Are the 13 inferred relationships involving `NewHand()` (e.g. with `TestBuildHandState()` and `TestBuildHandStatePerspectiveIndependentOfActor()`) actually correct?**
  _`NewHand()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/haohanwu/showdown`, `stub.sh script`, `Record` to the rest of the system?**
  _28 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Poker Engine Design Rationale` be split into smaller, more focused modules?**
  _Cohesion score 0.08143939393939394 - nodes in this community are weakly interconnected._
- **Should `Agent Adapter & Roster Detection` be split into smaller, more focused modules?**
  _Cohesion score 0.11083743842364532 - nodes in this community are weakly interconnected._