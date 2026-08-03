#!/usr/bin/env python3
"""Generate token-usage charts for the README from real showdown match data.

Requires: pip install -r requirements.txt

Usage:
  python3 plot_token_usage.py --debug ~/.showdown/debug-<ts>.jsonl \
      --compare MATCH_A.jsonl MATCH_B.jsonl --out ../docs/
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


import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt


def render_per_hand_chart(calls, out_path):
    """Stacked bar of tokens per decision: fresh input, cache-read input, output."""
    plt.style.use("dark_background")
    x = list(range(1, len(calls) + 1))
    fresh_in = [c["tokens_in"] - c["cache_read"] for c in calls]
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
