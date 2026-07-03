#!/usr/bin/env python3
"""Prompt spike: fire canned poker hand states at real agent CLIs.

Measures: JSON parse rate, action legality, latency, and (by eyeball)
poker quality + table-talk quality.

Usage:
  python3 run_spike.py [--agents claude,codex] [--only scenario-name] [--workers 4]
"""

import argparse
import concurrent.futures as cf
import json
import re
import subprocess
import sys
import time
from pathlib import Path

HERE = Path(__file__).parent
TEMPLATE = (HERE / "prompt_template.md").read_text()

AGENTS = {
    "claude": {
        "cmd": lambda prompt: ["/Users/haohanwu/.local/bin/claude", "-p", prompt],
        "name": "Claude Code",
    },
    "codex": {
        "cmd": lambda prompt: ["/Users/haohanwu/.local/bin/codex", "exec", prompt],
        "name": "Codex",
    },
}

TIMEOUT = 180


def build_prompt(agent_name: str, sc: dict) -> str:
    return (
        TEMPLATE
        .replace("{agent_name}", agent_name)
        .replace("{match_digest}", sc["match_digest"])
        .replace("{hand_state}", sc["hand_state"])
        .replace("{talk_log}", sc["talk_log"])
        .replace("{legal_actions}", sc["legal_actions"])
        .replace("{min_raise}", str(sc["min_raise"]))
        .replace("{max_amount}", str(sc["max_amount"]))
        .replace("{{", "{")
        .replace("}}", "}")
    )


def extract_json(text: str):
    """Find the last parseable JSON object with an 'action' key."""
    candidates = []
    for m in re.finditer(r"\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}", text, re.DOTALL):
        try:
            obj = json.loads(m.group())
            if isinstance(obj, dict) and "action" in obj:
                candidates.append(obj)
        except json.JSONDecodeError:
            pass
    return candidates[-1] if candidates else None


def run_one(agent_key: str, sc: dict) -> dict:
    agent = AGENTS[agent_key]
    prompt = build_prompt(agent["name"], sc)
    t0 = time.time()
    try:
        proc = subprocess.run(
            agent["cmd"](prompt),
            capture_output=True, text=True, timeout=TIMEOUT,
            cwd=str(HERE.parent),
        )
        raw = proc.stdout
        err = proc.stderr
    except subprocess.TimeoutExpired:
        return {"agent": agent_key, "scenario": sc["name"], "ok": False,
                "error": f"timeout {TIMEOUT}s", "latency": TIMEOUT}
    latency = round(time.time() - t0, 1)

    parsed = extract_json(raw)
    if parsed is None:
        return {"agent": agent_key, "scenario": sc["name"], "ok": False,
                "error": "no JSON with 'action' found", "latency": latency,
                "raw": raw[-500:], "stderr": err[-200:]}

    action = str(parsed.get("action", "")).lower()
    legal = [a.strip() for a in sc["legal_actions"].split(",")]
    legal_ok = action in legal
    amount = parsed.get("amount")
    amount_ok = True
    if action in ("bet", "raise"):
        amount_ok = (isinstance(amount, (int, float))
                     and sc["min_raise"] <= amount <= sc["max_amount"])

    return {"agent": agent_key, "scenario": sc["name"], "ok": True,
            "action": action, "amount": amount, "say": parsed.get("say"),
            "legal": legal_ok, "amount_ok": amount_ok,
            "expect": sc["expect"], "latency": latency}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--agents", default="claude,codex")
    ap.add_argument("--only", default=None)
    ap.add_argument("--workers", type=int, default=4)
    args = ap.parse_args()

    scenarios = json.loads((HERE / "scenarios.json").read_text())
    if args.only:
        scenarios = [s for s in scenarios if s["name"] == args.only]
    agent_keys = [a for a in args.agents.split(",") if a in AGENTS]

    jobs = [(a, s) for a in agent_keys for s in scenarios]
    print(f"Running {len(jobs)} calls ({len(agent_keys)} agents x {len(scenarios)} scenarios)...",
          file=sys.stderr)

    results = []
    with cf.ThreadPoolExecutor(max_workers=args.workers) as ex:
        futs = {ex.submit(run_one, a, s): (a, s["name"]) for a, s in jobs}
        for fut in cf.as_completed(futs):
            r = fut.result()
            results.append(r)
            tag = "OK " if r["ok"] else "ERR"
            print(f"  [{tag}] {r['agent']:6s} {r['scenario']:26s} {r.get('latency','?')}s",
                  file=sys.stderr)

    results.sort(key=lambda r: (r["agent"], r["scenario"]))
    out = HERE / "results.json"
    out.write_text(json.dumps(results, indent=2))

    # summary table
    print(f"\n{'agent':7s} {'scenario':26s} {'action':7s} {'amt':>6s} "
          f"{'legal':5s} {'expect':7s} {'sec':>5s}  say")
    for r in results:
        if r["ok"]:
            hit = "?" if r["expect"] == "any" else ("Y" if r["action"] == r["expect"] else "N")
            legal = "yes" if r["legal"] and r["amount_ok"] else "NO"
            amt = str(r["amount"]) if r["amount"] is not None else "-"
            say = (r["say"] or "")[:60]
            print(f"{r['agent']:7s} {r['scenario']:26s} {r['action']:7s} {amt:>6s} "
                  f"{legal:5s} {r['expect']+'/'+hit:7s} {r['latency']:>5.1f}  {say}")
        else:
            print(f"{r['agent']:7s} {r['scenario']:26s} FAILED: {r['error']}")

    n_ok = sum(1 for r in results if r["ok"])
    print(f"\nparse rate: {n_ok}/{len(results)}   full results -> {out}")


if __name__ == "__main__":
    main()
