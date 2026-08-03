# Token Usage Visualizations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two matplotlib charts to the README's Costs section, generated from real match data, backing the existing "roughly 10x cheaper" cache-warm claim and the haiku-vs-sonnet cost tradeoff.

**Architecture:** A single new Python script, `scripts/plot_token_usage.py`, with pure parsing functions (unit-testable via stdlib `unittest`, no matplotlib needed) separated from rendering functions (smoke-tested: produce a non-empty PNG). The script is run manually against the `--debug` JSONL transcripts of two short real matches (one per model), producing two PNGs committed to `docs/`. README gets two `![]()` embeds in the Costs section.

**Tech Stack:** Python 3 stdlib (`argparse`, `json`, `unittest`) + `matplotlib` (new dependency, not currently used anywhere in this repo — install via `pip install matplotlib`).

**Spec:** `docs/superpowers/specs/2026-07-21-token-usage-viz-design.md`

## Global Constraints

- Cross-model chart compares exactly two real matches, one played `--model sonnet` and one `--model haiku` (matches README's "Choosing a model (Claude)" scope) — not codex/gemini.
- `~/.showdown/stats.json` is NOT a usable data source for cross-model comparison: it's keyed by adapter (`claude`/`codex`/`gemini`), not by model, so a haiku match and a sonnet match both accumulate into the same `"claude"` record. Do not read it.
- Charts use `plt.style.use('dark_background')`. No ASCII/terminal-font styling.
- Output files, exact paths: `docs/token-usage-per-hand.png`, `docs/token-usage-by-model.png`.
- Chart generation is manual/one-off, not wired into CI — this repo has no `.github` workflows and none should be added.
- `agent_call` JSONL record fields (from `internal/debuglog/asker.go`): `seq`, `tokens_in`, `tokens_out`, `cache_read`, `cache_write`, `cost_usd` (plus `t`, `event`, `prompt`, `raw`, `duration_ms`, not used here).
- `tokens_in` is already the fresh/non-cached token count — it does NOT include `cache_read` or `cache_write`. Per `internal/agent/response.go`, `TotalIn() = InputTokens + CacheRead + CacheWrite` (three separate additive buckets). Never subtract `cache_read` from `tokens_in`.
- `session_start` JSONL record (from `internal/tui/app.go`, one per match, first line): has a `model` field (e.g. `"sonnet"`, `"haiku"`) among others (`agent_key`, `agent_name`, `dir`, `hand_limit`, `personality`, ...).
- The cross-model chart's totals are per-match sums, not career totals — axis/legend labels say "this match" explicitly.
- README caption text, exact (from spec): `*Cache-read tokens (cheap) dominate after decision 1.*` and `*One match each, haiku vs sonnet.*`.

---

### Task 1: `parse_agent_calls` — read one match's debug JSONL

**Files:**
- Create: `scripts/plot_token_usage.py`
- Create: `scripts/requirements.txt`
- Test: `scripts/test_plot_token_usage.py`

**Interfaces:**
- Produces: `parse_agent_calls(jsonl_path: str) -> list[dict]`. Each dict has keys `seq` (int), `tokens_in` (int), `tokens_out` (int), `cache_read` (int), `cache_write` (int), `cost_usd` (float). Only lines with `"event": "agent_call"` are included. Returned list is sorted by `seq`. Missing numeric fields default to `0` / `0.0`.

- [ ] **Step 1: Create `scripts/requirements.txt`**

```
matplotlib
```

- [ ] **Step 2: Install matplotlib locally**

Run: `pip install -r scripts/requirements.txt`
Expected: matplotlib installs without error (`python3 -c "import matplotlib; print(matplotlib.__version__)"` prints a version).

- [ ] **Step 3: Write the failing test for `parse_agent_calls`**

Create `scripts/test_plot_token_usage.py`:

```python
import json
import os
import tempfile
import unittest

from plot_token_usage import parse_agent_calls


class TestParseAgentCalls(unittest.TestCase):
    def _write_jsonl(self, lines):
        f = tempfile.NamedTemporaryFile(
            mode="w", suffix=".jsonl", delete=False
        )
        for line in lines:
            f.write(json.dumps(line) + "\n")
        f.close()
        self.addCleanup(os.unlink, f.name)
        return f.name

    def test_extracts_and_orders_agent_call_events(self):
        path = self._write_jsonl(
            [
                {
                    "seq": 2,
                    "event": "agent_call",
                    "tokens_in": 500,
                    "tokens_out": 80,
                    "cache_read": 400,
                    "cache_write": 0,
                    "cost_usd": 0.002,
                },
                {"seq": 1, "event": "hand_start"},
                {
                    "seq": 3,
                    "event": "agent_call",
                    "tokens_in": 2000,
                    "tokens_out": 90,
                    "cache_read": 0,
                    "cache_write": 1900,
                    "cost_usd": 0.02,
                },
            ]
        )
        result = parse_agent_calls(path)
        self.assertEqual([r["seq"] for r in result], [2, 3])
        self.assertEqual(result[0]["tokens_in"], 500)
        self.assertEqual(result[0]["cache_read"], 400)
        self.assertEqual(result[1]["cache_write"], 1900)

    def test_defaults_missing_numeric_fields_to_zero(self):
        path = self._write_jsonl(
            [{"seq": 1, "event": "agent_call"}]
        )
        result = parse_agent_calls(path)
        self.assertEqual(
            result[0],
            {
                "seq": 1,
                "tokens_in": 0,
                "tokens_out": 0,
                "cache_read": 0,
                "cache_write": 0,
                "cost_usd": 0.0,
            },
        )


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 4: Run test to verify it fails**

Run: `cd scripts && python3 -m unittest test_plot_token_usage -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'plot_token_usage'` (file doesn't exist yet).

- [ ] **Step 5: Write minimal implementation**

Create `scripts/plot_token_usage.py`:

```python
#!/usr/bin/env python3
"""Generate token-usage charts for the README from real showdown match data.

Requires: pip install -r requirements.txt

Usage:
  python3 plot_token_usage.py --debug ~/.showdown/debug-<ts>.jsonl \
      --stats ~/.showdown/stats.json --out ../docs/
"""

import argparse
import json


def parse_agent_calls(jsonl_path):
    """Read one match's --debug JSONL, return agent_call records sorted by seq."""
    records = []
    with open(jsonl_path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            rec = json.loads(line)
            if rec.get("event") != "agent_call":
                continue
            records.append(
                {
                    "seq": rec["seq"],
                    "tokens_in": rec.get("tokens_in", 0),
                    "tokens_out": rec.get("tokens_out", 0),
                    "cache_read": rec.get("cache_read", 0),
                    "cache_write": rec.get("cache_write", 0),
                    "cost_usd": rec.get("cost_usd", 0.0),
                }
            )
    records.sort(key=lambda r: r["seq"])
    return records
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd scripts && python3 -m unittest test_plot_token_usage -v`
Expected: `OK` (2 tests pass).

- [ ] **Step 7: Commit**

```bash
git add scripts/plot_token_usage.py scripts/test_plot_token_usage.py scripts/requirements.txt
git commit -m "feat: parse agent_call records from debug JSONL for token-usage charts"
```

---

### Task 2: `read_match_model` + `summarize_match` — per-match model label and totals

**Revision note:** this task originally read career totals from
`~/.showdown/stats.json` keyed by `haiku`/`sonnet`. That data doesn't
exist — `stats.json` keys by adapter (`claude`/`codex`/`gemini`), not
model, discovered during real data collection (Task 6). This version
reads the two match debug JSONLs directly instead.

**Files:**
- Modify: `scripts/plot_token_usage.py`
- Test: `scripts/test_plot_token_usage.py`

**Interfaces:**
- Consumes: `parse_agent_calls` from Task 1 (same module).
- Produces: `read_match_model(jsonl_path: str) -> str`. Reads the JSONL's `session_start` event and returns its `model` field; returns `"unknown"` if no `session_start` event is found or the field is absent.
- Produces: `summarize_match(jsonl_path: str) -> dict`. Returns `{"tokens_in": int, "tokens_out": int, "cost_usd": float}` — the sum of those fields across all `agent_call` records in the file (via `parse_agent_calls`). An empty/no-call file returns `{"tokens_in": 0, "tokens_out": 0, "cost_usd": 0.0}`.

- [ ] **Step 1: Write the failing tests**

Add to `scripts/test_plot_token_usage.py` (new import line and new test class):

```python
from plot_token_usage import (
    parse_agent_calls,
    read_match_model,
    summarize_match,
)


class TestReadMatchModel(unittest.TestCase):
    def _write_jsonl(self, lines):
        f = tempfile.NamedTemporaryFile(
            mode="w", suffix=".jsonl", delete=False
        )
        for line in lines:
            f.write(json.dumps(line) + "\n")
        f.close()
        self.addCleanup(os.unlink, f.name)
        return f.name

    def test_reads_model_from_session_start(self):
        path = self._write_jsonl(
            [
                {"seq": 1, "event": "session_start", "model": "sonnet"},
                {"seq": 2, "event": "agent_call", "tokens_in": 100},
            ]
        )
        self.assertEqual(read_match_model(path), "sonnet")

    def test_returns_unknown_when_no_session_start(self):
        path = self._write_jsonl(
            [{"seq": 1, "event": "agent_call", "tokens_in": 100}]
        )
        self.assertEqual(read_match_model(path), "unknown")


class TestSummarizeMatch(unittest.TestCase):
    def _write_jsonl(self, lines):
        f = tempfile.NamedTemporaryFile(
            mode="w", suffix=".jsonl", delete=False
        )
        for line in lines:
            f.write(json.dumps(line) + "\n")
        f.close()
        self.addCleanup(os.unlink, f.name)
        return f.name

    def test_sums_tokens_and_cost_across_calls(self):
        path = self._write_jsonl(
            [
                {"seq": 1, "event": "session_start", "model": "haiku"},
                {
                    "seq": 2,
                    "event": "agent_call",
                    "tokens_in": 2000,
                    "tokens_out": 90,
                    "cache_read": 0,
                    "cache_write": 1900,
                    "cost_usd": 0.02,
                },
                {
                    "seq": 3,
                    "event": "agent_call",
                    "tokens_in": 2100,
                    "tokens_out": 85,
                    "cache_read": 2000,
                    "cache_write": 0,
                    "cost_usd": 0.003,
                },
            ]
        )
        result = summarize_match(path)
        self.assertEqual(
            result,
            {"tokens_in": 4100, "tokens_out": 175, "cost_usd": 0.023},
        )

    def test_empty_file_sums_to_zero(self):
        path = self._write_jsonl(
            [{"seq": 1, "event": "session_start", "model": "sonnet"}]
        )
        result = summarize_match(path)
        self.assertEqual(
            result, {"tokens_in": 0, "tokens_out": 0, "cost_usd": 0.0}
        )
```

(Replace the existing `from plot_token_usage import parse_agent_calls` line with the combined import shown above.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd scripts && python3 -m unittest test_plot_token_usage -v`
Expected: FAIL — `ImportError: cannot import name 'read_match_model'`.

- [ ] **Step 3: Write minimal implementation**

Add to `scripts/plot_token_usage.py`, after `parse_agent_calls`:

```python
def read_match_model(jsonl_path):
    """Read a match's session_start event, return its model field."""
    with open(jsonl_path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            rec = json.loads(line)
            if rec.get("event") == "session_start":
                return rec.get("model", "unknown")
    return "unknown"


def summarize_match(jsonl_path):
    """Sum tokens_in, tokens_out, cost_usd across a match's agent_call records."""
    calls = parse_agent_calls(jsonl_path)
    return {
        "tokens_in": sum(c["tokens_in"] for c in calls),
        "tokens_out": sum(c["tokens_out"] for c in calls),
        "cost_usd": sum(c["cost_usd"] for c in calls),
    }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd scripts && python3 -m unittest test_plot_token_usage -v`
Expected: `OK` (6 tests pass).

- [ ] **Step 5: Commit**

```bash
git add scripts/plot_token_usage.py scripts/test_plot_token_usage.py
git commit -m "feat: read per-match model and token/cost totals from debug JSONL"
```

---

### Task 3: `render_per_hand_chart`

**Revision note:** originally stacked `tokens_in - cache_read` as the
"fresh input" segment. That's wrong: the Claude usage API's
`input_tokens` field is *already* the fresh (non-cached) token count —
it does not include `cache_read_input_tokens` or
`cache_creation_input_tokens`. Confirmed in `internal/agent/response.go`:
`TotalIn() = InputTokens + CacheRead + CacheWrite` — three separate
buckets summed, not nested. Subtracting `cache_read` from `tokens_in`
double-counts the cache savings and goes negative whenever
`cache_read` is large relative to `tokens_in` (the exact "cache
warm" case this chart exists to show — discovered on real match data
where `tokens_in` was ~1 and `cache_read` was in the thousands).
Fixed by stacking `tokens_in` directly, no subtraction.

**Files:**
- Modify: `scripts/plot_token_usage.py`
- Test: `scripts/test_plot_token_usage.py`

**Interfaces:**
- Consumes: `parse_agent_calls` output shape (list of dicts with `seq`, `tokens_in`, `tokens_out`, `cache_read`).
- Produces: `render_per_hand_chart(calls: list[dict], out_path: str) -> None`. Writes a PNG to `out_path`. Stacked bar per decision: `tokens_in` (fresh, already excludes cache) + `cache_read` (cached) + `tokens_out`, x-axis = decision index (1-based position in the list, not raw `seq`), dark background style.

- [ ] **Step 1: Write the failing test (smoke test — matplotlib output isn't pixel-diffed, just checked for existence and non-zero size)**

Add to `scripts/test_plot_token_usage.py`:

```python
from plot_token_usage import parse_agent_calls, parse_stats, render_per_hand_chart


class TestRenderPerHandChart(unittest.TestCase):
    def test_writes_nonempty_png(self):
        calls = [
            {"seq": 1, "tokens_in": 2000, "tokens_out": 90,
             "cache_read": 0, "cache_write": 1900, "cost_usd": 0.02},
            {"seq": 2, "tokens_in": 2100, "tokens_out": 85,
             "cache_read": 2000, "cache_write": 0, "cost_usd": 0.003},
        ]
        out = tempfile.NamedTemporaryFile(suffix=".png", delete=False)
        out.close()
        self.addCleanup(os.unlink, out.name)

        render_per_hand_chart(calls, out.name)

        self.assertTrue(os.path.getsize(out.name) > 0)
```

(Replace the import line again to add `render_per_hand_chart`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd scripts && python3 -m unittest test_plot_token_usage -v`
Expected: FAIL — `ImportError: cannot import name 'render_per_hand_chart'`.

- [ ] **Step 3: Write minimal implementation**

Add to `scripts/plot_token_usage.py`, after `parse_stats`:

```python
import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt


def render_per_hand_chart(calls, out_path):
    """Stacked bar of tokens per decision: fresh input, cache-read input, output."""
    plt.style.use("dark_background")
    x = list(range(1, len(calls) + 1))
    fresh_in = [c["tokens_in"] for c in calls]
    cache_read = [c["cache_read"] for c in calls]
    tokens_out = [c["tokens_out"] for c in calls]

    fig, ax = plt.subplots(figsize=(8, 4.5))
    ax.bar(x, fresh_in, label="fresh input tokens", color="#e07a5f")
    ax.bar(x, cache_read, bottom=fresh_in, label="cache-read tokens", color="#3d5a80")
    bottom_out = [f + c for f, c in zip(fresh_in, cache_read)]
    ax.bar(x, tokens_out, bottom=bottom_out, label="output tokens", color="#81b29a")

    ax.set_xlabel("decision number")
    ax.set_ylabel("tokens")
    ax.set_title("Token usage per decision (one match)")
    ax.set_xticks(x)
    ax.legend()
    fig.tight_layout()
    fig.savefig(out_path, dpi=150)
    plt.close(fig)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd scripts && python3 -m unittest test_plot_token_usage -v`
Expected: `OK` (5 tests pass).

- [ ] **Step 5: Commit**

```bash
git add scripts/plot_token_usage.py scripts/test_plot_token_usage.py
git commit -m "feat: render per-hand token-usage chart"
```

---

### Task 4: `render_cross_model_chart`

**Revision note:** originally labeled "career total" (matching the
now-abandoned `parse_stats`/`stats.json` design). Task 2 was reworked
to produce per-match totals instead, so this task's axis/title/legend
text changes from "career total" to "this match" to match. The
function's input shape (`{key: {"tokens_in", "tokens_out",
"cost_usd"}}`) is unchanged — it still doesn't care where the dict
came from.

**Files:**
- Modify: `scripts/plot_token_usage.py`
- Test: `scripts/test_plot_token_usage.py`

**Interfaces:**
- Consumes: a `dict[str, dict]` shaped like `summarize_match`'s per-match output, one entry per model (Task 2 produces the values; the caller in Task 5 supplies the model-name keys via `read_match_model`).
- Produces: `render_cross_model_chart(stats: dict, out_path: str) -> None`. Writes a PNG. Grouped bars, one group per key in `stats` (in insertion order), left axis = total tokens (`tokens_in + tokens_out`), right (twin) axis = `cost_usd`, both labeled "this match".

- [ ] **Step 1: Write the failing test**

Add to `scripts/test_plot_token_usage.py`:

```python
from plot_token_usage import (
    parse_agent_calls,
    read_match_model,
    summarize_match,
    render_per_hand_chart,
    render_cross_model_chart,
)


class TestRenderCrossModelChart(unittest.TestCase):
    def test_writes_nonempty_png(self):
        stats = {
            "haiku": {"tokens_in": 10000, "tokens_out": 2000, "cost_usd": 0.05},
            "sonnet": {"tokens_in": 8000, "tokens_out": 1500, "cost_usd": 0.40},
        }
        out = tempfile.NamedTemporaryFile(suffix=".png", delete=False)
        out.close()
        self.addCleanup(os.unlink, out.name)

        render_cross_model_chart(stats, out.name)

        self.assertTrue(os.path.getsize(out.name) > 0)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd scripts && python3 -m unittest test_plot_token_usage -v`
Expected: FAIL — `ImportError: cannot import name 'render_cross_model_chart'`.

- [ ] **Step 3: Write minimal implementation**

Add to `scripts/plot_token_usage.py`, after `render_per_hand_chart`:

```python
def render_cross_model_chart(stats, out_path):
    """Grouped bar: this-match total tokens (left axis) and cost_usd (right axis) per model."""
    plt.style.use("dark_background")
    keys = list(stats.keys())
    totals = [stats[k]["tokens_in"] + stats[k]["tokens_out"] for k in keys]
    costs = [stats[k]["cost_usd"] for k in keys]
    x = list(range(len(keys)))
    width = 0.35

    fig, ax1 = plt.subplots(figsize=(6, 4.5))
    ax1.bar([i - width / 2 for i in x], totals, width, color="#3d5a80",
            label="this match: tokens")
    ax1.set_ylabel("this match: total tokens")
    ax1.set_xticks(x)
    ax1.set_xticklabels(keys)

    ax2 = ax1.twinx()
    ax2.bar([i + width / 2 for i in x], costs, width, color="#e07a5f",
            label="this match: cost ($)")
    ax2.set_ylabel("this match: cost (USD)")

    ax1.set_title("Token usage and cost by model (one match each)")
    lines1, labels1 = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax1.legend(lines1 + lines2, labels1 + labels2, loc="upper right")
    fig.tight_layout()
    fig.savefig(out_path, dpi=150)
    plt.close(fig)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd scripts && python3 -m unittest test_plot_token_usage -v`
Expected: `OK` (8 tests pass).

- [ ] **Step 5: Commit**

```bash
git add scripts/plot_token_usage.py scripts/test_plot_token_usage.py
git commit -m "feat: render cross-model per-match token-usage chart"
```

---

### Task 5: CLI wiring (`main`)

**Revision note:** originally took `--stats ~/.showdown/stats.json`.
Replaced with `--compare MATCH_A MATCH_B` (two debug JSONL paths)
since `stats.json` can't distinguish models (see Task 2's revision
note). `--debug` (for the per-hand chart) is unchanged.

**Files:**
- Modify: `scripts/plot_token_usage.py`

**Interfaces:**
- Consumes: `parse_agent_calls`, `render_per_hand_chart` from Tasks 1/3; `read_match_model`, `summarize_match` from Task 2; `render_cross_model_chart` from Task 4.
- Produces: `main(argv=None) -> None`, and `if __name__ == "__main__": main()`. No new interfaces for later tasks — this is the last piece of the script.

- [ ] **Step 1: Add `main` and CLI entry point**

Add to `scripts/plot_token_usage.py`, at the end of the file:

```python
def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--debug", required=True, help="path to the featured match's --debug JSONL (used for the per-hand chart)")
    parser.add_argument("--compare", nargs=2, metavar=("MATCH_A", "MATCH_B"), required=True, help="two --debug JSONL paths from different models, for the cross-model chart")
    parser.add_argument("--out", required=True, help="output directory for the PNGs")
    args = parser.parse_args(argv)

    calls = parse_agent_calls(args.debug)
    render_per_hand_chart(calls, f"{args.out.rstrip('/')}/token-usage-per-hand.png")

    stats = {}
    for path in args.compare:
        model = read_match_model(path)
        stats[model] = summarize_match(path)
    render_cross_model_chart(stats, f"{args.out.rstrip('/')}/token-usage-by-model.png")


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Smoke-test the full CLI with fixture data**

Run:

```bash
cd scripts
mkdir -p /tmp/showdown-plot-smoke
cat > /tmp/showdown-plot-smoke/sonnet.jsonl <<'EOF'
{"seq":1,"event":"session_start","model":"sonnet"}
{"seq":2,"event":"agent_call","tokens_in":2000,"tokens_out":90,"cache_read":0,"cache_write":1900,"cost_usd":0.02}
{"seq":3,"event":"agent_call","tokens_in":2100,"tokens_out":85,"cache_read":2000,"cache_write":0,"cost_usd":0.003}
EOF
cat > /tmp/showdown-plot-smoke/haiku.jsonl <<'EOF'
{"seq":1,"event":"session_start","model":"haiku"}
{"seq":2,"event":"agent_call","tokens_in":1500,"tokens_out":60,"cache_read":0,"cache_write":1400,"cost_usd":0.004}
EOF
python3 plot_token_usage.py --debug /tmp/showdown-plot-smoke/sonnet.jsonl \
  --compare /tmp/showdown-plot-smoke/sonnet.jsonl /tmp/showdown-plot-smoke/haiku.jsonl \
  --out /tmp/showdown-plot-smoke
ls -la /tmp/showdown-plot-smoke/*.png
```

Expected: two PNG files listed with non-zero size, no traceback.

- [ ] **Step 3: Commit**

```bash
git add scripts/plot_token_usage.py
git commit -m "feat: wire plot_token_usage CLI (argparse main, --compare replaces --stats)"
```

---

### Task 6: Collect real match data and generate final charts

This task is manual/interactive — the game is a TUI played by a human, not scriptable headlessly. Whoever executes this task must actually sit down and play two short matches.

**Files:**
- None modified (this task produces data files under `~/.showdown/`, not repo files).

- [ ] **Step 1: Build showdown**

Run: `go build -o /tmp/showdown-bin .` (from repo root)
Expected: builds with no errors.

- [ ] **Step 2: Play a short match against sonnet with debug logging on**

Run: `/tmp/showdown-bin --debug --model sonnet --hands 5`

Play through 5 hands (any actions — fold, call, raise, whatever happens naturally). Let the match finish or bust out. On exit, note the printed debug transcript path, e.g. `~/.showdown/debug-20260721-153000.jsonl`.

- [ ] **Step 3: Play a short match against haiku**

Run: `/tmp/showdown-bin --debug --model haiku --hands 5`

Play through 5 hands the same way. Note this JSONL's path too — it's used directly by the cross-model chart (Task 2's revision: `stats.json` can't split by model, so both match JSONLs are read directly).

- [ ] **Step 4: Verify both JSONLs have a session_start model field**

Run: `head -1 ~/.showdown/debug-<sonnet-timestamp>.jsonl ~/.showdown/debug-<haiku-timestamp>.jsonl`
Expected: each first line is a `session_start` event with `"model":"sonnet"` and `"model":"haiku"` respectively.

- [ ] **Step 5: Generate the real charts into `docs/`**

Run (from repo root, substituting the actual debug paths from Steps 2-3):

```bash
python3 scripts/plot_token_usage.py \
  --debug ~/.showdown/debug-<sonnet-match-timestamp>.jsonl \
  --compare ~/.showdown/debug-<sonnet-match-timestamp>.jsonl ~/.showdown/debug-<haiku-match-timestamp>.jsonl \
  --out docs/
```

Expected: `docs/token-usage-per-hand.png` and `docs/token-usage-by-model.png` created, no traceback.

- [ ] **Step 6: Eyeball both PNGs**

Open both files (e.g. `open docs/token-usage-per-hand.png docs/token-usage-by-model.png` on macOS). Confirm: per-hand chart shows decision 1 with a large fresh-input bar and later decisions mostly cache-read; cross-model chart shows both haiku and sonnet bars with plausible (non-zero, non-identical) values.

- [ ] **Step 7: Commit the generated PNGs**

```bash
git add docs/token-usage-per-hand.png docs/token-usage-by-model.png
git commit -m "chore: generate token-usage charts from real match data"
```

---

### Task 7: Embed charts in README

**Files:**
- Modify: `README.md` (Costs section, currently ending "...under a dollar." — see line ~93)

**Interfaces:**
- Consumes: `docs/token-usage-per-hand.png`, `docs/token-usage-by-model.png` from Task 6.

- [ ] **Step 1: Read the current Costs section to confirm exact insertion point**

Run: `grep -n "under a dollar" README.md`
Expected: prints the line number of `10x cheaper). A typical sonnet match runs well` / `under a dollar.` paragraph.

- [ ] **Step 2: Insert the two images right after that paragraph, before "Quitting mid-match..."**

Find this text in `README.md`:

```
per match, so decisions after the first are prompt-cache warm (roughly
10x cheaper than one-shot calls). A typical sonnet match runs well
under a dollar.

Quitting mid-match skips the career-cost entry; the `--debug`
```

Replace with:

```
per match, so decisions after the first are prompt-cache warm (roughly
10x cheaper than one-shot calls). A typical sonnet match runs well
under a dollar.

![Token usage per hand](docs/token-usage-per-hand.png)
*Cache-read tokens (cheap) dominate after decision 1.*

![Token usage by model](docs/token-usage-by-model.png)
*One match each, haiku vs sonnet.*

Quitting mid-match skips the career-cost entry; the `--debug`
```

- [ ] **Step 3: Verify rendering**

Run: `grep -n "token-usage" README.md`
Expected: 2 `![]()` lines printed, pointing at `docs/token-usage-per-hand.png` and `docs/token-usage-by-model.png`.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: embed token-usage charts in Costs section"
```
