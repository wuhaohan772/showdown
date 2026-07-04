# Graph Report - .  (2026-07-04)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 465 nodes · 841 edges · 43 communities (26 shown, 17 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 101 edges (avg confidence: 0.81)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `a688375e`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Model|Model]]
- [[_COMMUNITY_Card|Card]]
- [[_COMMUNITY_GetDecision|GetDecision]]
- [[_COMMUNITY_app_test.go|app_test.go]]
- [[_COMMUNITY_Hand|Hand]]
- [[_COMMUNITY_hand_test.go|hand_test.go]]
- [[_COMMUNITY_LoadPersonality|LoadPersonality]]
- [[_COMMUNITY_adapter_test.go|adapter_test.go]]
- [[_COMMUNITY_Logger|Logger]]
- [[_COMMUNITY_Debug Transcript Log Design|Debug Transcript Log Design]]
- [[_COMMUNITY_Decision Request|Decision Request]]
- [[_COMMUNITY_Personality System Implementation Plan|Personality System Implementation Plan]]
- [[_COMMUNITY_Match|Match]]
- [[_COMMUNITY_Lean opponent sessions|Lean opponent sessions]]
- [[_COMMUNITY_Load|Load]]
- [[_COMMUNITY_Poker Strategy|Poker Strategy]]
- [[_COMMUNITY_Usage|Usage]]
- [[_COMMUNITY_RenderPrompt|RenderPrompt]]
- [[_COMMUNITY_Usage Struct|Usage Struct]]
- [[_COMMUNITY_pickerRoster|pickerRoster]]
- [[_COMMUNITY_Digest|Digest]]
- [[_COMMUNITY_JSON Response Format|JSON Response Format]]
- [[_COMMUNITY_Key Hint Row|Key Hint Row]]
- [[_COMMUNITY_run_spike.py|run_spike.py]]
- [[_COMMUNITY_TokenCost Measurement + Model Choice Implementation Plan|Token/Cost Measurement + Model Choice Implementation Plan]]
- [[_COMMUNITY_CLAUDE|CLAUDE.md]]
- [[_COMMUNITY_Claude System Prompt Cache Constraint|Claude System Prompt Cache Constraint]]
- [[_COMMUNITY_Claude Code persona|Claude Code persona]]
- [[_COMMUNITY_stub.sh|stub.sh]]
- [[_COMMUNITY_stub-agent.sh|stub-agent.sh]]
- [[_COMMUNITY_Action|Action]]
- [[_COMMUNITY_ActionType|ActionType]]
- [[_COMMUNITY_Roster|Roster]]
- [[_COMMUNITY_Hand|Hand]]
- [[_COMMUNITY_Chip Management|Chip Management]]
- [[_COMMUNITY_Logger|Logger]]
- [[_COMMUNITY_Rand|Rand]]
- [[_COMMUNITY_Adapter|Adapter]]
- [[_COMMUNITY_T|T]]
- [[_COMMUNITY_github.comhaohanwushowdown|github.com/haohanwu/showdown]]
- [[_COMMUNITY_RequestData|RequestData]]
- [[_COMMUNITY_Stats|Stats]]
- [[_COMMUNITY_Usage|Usage]]

## God Nodes (most connected - your core abstractions)
1. `Model` - 28 edges
2. `Hand` - 24 edges
3. `testModel()` - 24 edges
4. `key()` - 19 edges
5. `GetDecision()` - 17 edges
6. `NewHand()` - 16 edges
7. `Card` - 15 edges
8. `mustApply()` - 13 edges
9. `newTestHand()` - 12 edges
10. `NewModel()` - 12 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `NewModel()`  [INFERRED]
  main.go → internal/tui/app.go
- `main()` --calls--> `PersonalityPath()`  [INFERRED]
  main.go → internal/agent/personality.go
- `main()` --calls--> `RunPicker()`  [INFERRED]
  main.go → internal/tui/picker.go
- `main()` --calls--> `LoadPersonality()`  [INFERRED]
  main.go → internal/agent/personality.go
- `PersonalityCap Constant` --references--> `User Personality File`  [INFERRED]
  docs/superpowers/plans/2026-07-04-personality-system.md → README.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Usage Data Flow** — docs_superpowers_plans_2026_07_03_token_cost_and_model_choice_usage_struct, docs_superpowers_plans_2026_07_03_token_cost_and_model_choice_debug_log, docs_superpowers_plans_2026_07_03_token_cost_and_model_choice_tui_model, docs_superpowers_plans_2026_07_03_token_cost_and_model_choice_stats_record [INFERRED 0.85]
- **Key Hint Row View Phase Switch Implementation** — docs_superpowers_specs_2026_07_03_key_hints_design_key_hint_row, docs_superpowers_specs_2026_07_03_key_hints_design_phase_agent_turn_hint, docs_superpowers_specs_2026_07_03_key_hints_design_phase_hand_end_hint, internal_tui_app_view [EXTRACTED 1.00]
- **Debug Event Capture Pipeline** — docs_superpowers_specs_2026_07_03_debug_transcript_log_design_logger, docs_superpowers_specs_2026_07_03_debug_transcript_log_design_wrapasker, docs_superpowers_specs_2026_07_03_debug_transcript_log_design_apply_helper [EXTRACTED 1.00]
- **Phase-Aware Talk Input Management** — docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_talkreturn_field, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_opentalk_helper, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_phase_widening [EXTRACTED 1.00]
- **Async State Preservation Pattern** — docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_talkreturn_field, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_decision_race_handling, docs_superpowers_specs_2026_07_03_talk_echo_and_phases_design_runout_tick_handling [EXTRACTED 1.00]
- **Decision Pipeline: Request → Decision → Action** — context_md_decision_request, context_md_decision, context_md_fallback_action [INFERRED 0.85]
- **ADR Implementation in Code** — docs_adr_0001_stateless_decision_requests, docs_adr_0002_native_memory_loading, superpowers_sdd_task_9_report_get_decision, superpowers_sdd_task_10_brief_asker_method [INFERRED 0.75]

## Communities (43 total, 17 thin omitted)

### Community 0 - "Model"
Cohesion: 0.10
Nodes (27): Card, Cmd, Digest, agentHandSummary(), cardStrings(), Action, ActionType, Adapter (+19 more)

### Community 1 - "Card"
Cohesion: 0.12
Nodes (30): Rand, NewDeck(), ParseCard(), T, TestCardString(), TestDeckDeals52Unique(), TestDeckShuffled(), TestParseCardRoundTrip() (+22 more)

### Community 2 - "GetDecision"
Cohesion: 0.13
Nodes (32): Asker, Decision, Context, ExtractDecision(), FallbackAction(), GetDecision(), GetReaction(), Action (+24 more)

### Community 3 - "app_test.go"
Cohesion: 0.19
Nodes (34): NewModel(), KeyMsg, T, key(), readLogEvents(), TestAgentDecisionLogged(), TestAgentDecisionMsgApplies(), TestAgentHandSummaryPerspective() (+26 more)

### Community 4 - "Hand"
Cohesion: 0.18
Nodes (7): Action, ActionType, Hand, LogItem, Result, SeatState, Street

### Community 5 - "hand_test.go"
Cohesion: 0.26
Nodes (22): Rand, NewHand(), T, hasAction(), mustApply(), newTestHand(), TestAllInActorHasNoActions(), TestAllInCallRunsOutBoard() (+14 more)

### Community 6 - "LoadPersonality"
Cohesion: 0.16
Nodes (17): loadFile(), LoadPersonality(), PersonalityPath(), PresetNames(), T, TestLoadPersonalityDefaultFileWins(), TestLoadPersonalityDefaultsToNeedler(), TestLoadPersonalityErrors() (+9 more)

### Community 7 - "adapter_test.go"
Cohesion: 0.18
Nodes (16): Adapter, Asker, Duration, DetectRoster(), T, TestAskerAppliesEnv(), TestAskerAppliesParse(), TestAskerTimeout() (+8 more)

### Community 8 - "Logger"
Cohesion: 0.20
Nodes (13): Logger, File, DefaultPath(), New(), T, readEvents(), TestDefaultPath(), TestLogAfterCloseIsNoop() (+5 more)

### Community 9 - "Debug Transcript Log Design"
Cohesion: 0.17
Nodes (17): Debug Transcript Log Implementation Plan, Trash-Talk Echo & Phase Widening Implementation Plan, apply Helper, Bug Report Protocol, Debug Transcript Log Design, internal/debuglog Package, JSONL Format, Logger (+9 more)

### Community 10 - "Decision Request"
Cohesion: 0.15
Nodes (16): Agent Adapter, Career Record, Decision, Decision Request, Fallback Action, Game, Hand-End Reaction, Match (+8 more)

### Community 11 - "Personality System Implementation Plan"
Cohesion: 0.13
Nodes (16): LoadPersonality Function, main.go Integration, PersonalityCap Constant, personality.go File, PersonalityPath Function, Personality System Implementation Plan, PresetNames Function, Match-end Reaction Prompt (+8 more)

### Community 12 - "Match"
Cohesion: 0.22
Nodes (6): NewMatch(), T, TestBlindSchedule(), TestButtonAlternates(), TestNextHandAndBust(), Match

### Community 13 - "Lean opponent sessions"
Cohesion: 0.15
Nodes (14): ADR-0003: Lean opponent sessions, Claude adapter, Codex adapters, Context token reduction, Cost per decision reduction, Default system prompt, --disallowedTools flag, Gemini adapters (+6 more)

### Community 14 - "Load"
Cohesion: 0.22
Nodes (9): Load(), T, TestLine(), TestLineIncludesCost(), TestLoadMissingFileIsEmpty(), TestRecordUsageRoundTrip(), TestSaveLoadRoundTrip(), Record (+1 more)

### Community 15 - "Poker Strategy"
Cohesion: 0.17
Nodes (13): Betting History, Bluffing, Card Talk Doctrine, Coding Agent, No-Limit Texas Hold'em, Personal Needling, Poker Strategy, Position (+5 more)

### Community 16 - "Usage"
Cohesion: 0.24
Nodes (8): claudeEnvelope, Response, Usage, ParseClaudeJSON(), T, TestParseClaudeJSON(), TestParseClaudeJSONMalformedFallsBackToRaw(), TestUsageAdd()

### Community 17 - "RenderPrompt"
Cohesion: 0.29
Nodes (10): RequestData, BuildHandState(), Hand, RenderPrompt(), T, TestBuildHandState(), TestBuildHandStatePerspectiveIndependentOfActor(), TestDigestFlow() (+2 more)

### Community 18 - "Usage Struct"
Cohesion: 0.23
Nodes (12): Adapter Struct, Asker Type, Claude CLI, Debug Log Usage, GetDecision Function, GetReaction Function, JSON Output Format, ParseClaudeJSON Function (+4 more)

### Community 19 - "pickerRoster"
Cohesion: 0.44
Nodes (9): Adapter, pickerRoster(), TestPickerModelDefaultOnEmpty(), TestPickerModelSelection(), TestPickerPersonalityDefaultOnEmpty(), TestPickerPersonalitySelection(), TestPickerPresetModelSkipsPrompt(), TestPickerPresetPersonalitySkipsPrompt() (+1 more)

### Community 21 - "JSON Response Format"
Cohesion: 0.32
Nodes (8): Action Field, Amount Field, Hand State, JSON Response Format, Legal Actions, Match Digest, Say Field, Talk Log

### Community 22 - "Key Hint Row"
Cohesion: 0.29
Nodes (7): Key Hint Rows Implementation Plan, TestKeyHintRows, Banner Cleanup, Discoverability Gap, Key Hint Row, Phase Agent Turn Hint Case, Phase Hand End Hint Case

### Community 23 - "run_spike.py"
Cohesion: 0.53
Nodes (5): build_prompt(), extract_json(), main(), Find the last parseable JSON object with an 'action' key., run_one()

### Community 24 - "Token/Cost Measurement + Model Choice Implementation Plan"
Cohesion: 0.50
Nodes (4): ADR-0001 Invariant, Model Choice, Token/Cost Measurement + Model Choice Implementation Plan, Token/Cost Measurement

### Community 25 - "CLAUDE.md"
Cohesion: 0.67
Nodes (3): ADR-0002, CLAUDE.md, Memory files

### Community 26 - "Claude System Prompt Cache Constraint"
Cohesion: 0.67
Nodes (3): ADR-0003 Prompt Cache, Claude System Prompt Cache Constraint, Personality Template Slot

## Knowledge Gaps
- **55 isolated node(s):** `github.com/haohanwu/showdown`, `stub.sh script`, `stub-agent.sh script`, `Game`, `Roster` (+50 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **17 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Model` connect `Model` to `app_test.go`?**
  _High betweenness centrality (0.043) - this node is a cross-community bridge._
- **Why does `NewModel()` connect `app_test.go` to `Model`, `LoadPersonality`?**
  _High betweenness centrality (0.029) - this node is a cross-community bridge._
- **Why does `GetDecision()` connect `GetDecision` to `Model`, `RenderPrompt`?**
  _High betweenness centrality (0.028) - this node is a cross-community bridge._
- **What connects `github.com/haohanwu/showdown`, `stub.sh script`, `stub-agent.sh script` to the rest of the system?**
  _62 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Model` be split into smaller, more focused modules?**
  _Cohesion score 0.10188261351052048 - nodes in this community are weakly interconnected._
- **Should `Card` be split into smaller, more focused modules?**
  _Cohesion score 0.11522048364153627 - nodes in this community are weakly interconnected._
- **Should `GetDecision` be split into smaller, more focused modules?**
  _Cohesion score 0.1253968253968254 - nodes in this community are weakly interconnected._