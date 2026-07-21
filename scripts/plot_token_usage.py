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


def parse_stats(stats_path, keys):
    """Read stats.json, return career totals for the requested opponent keys."""
    with open(stats_path) as f:
        all_stats = json.load(f)
    result = {}
    for key in keys:
        if key not in all_stats:
            continue
        rec = all_stats[key]
        result[key] = {
            "tokens_in": rec.get("tokens_in", 0),
            "tokens_out": rec.get("tokens_out", 0),
            "cost_usd": rec.get("cost_usd", 0.0),
        }
    return result


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
    """Grouped bar: career total tokens (left axis) and cost_usd (right axis) per model."""
    plt.style.use("dark_background")
    keys = list(stats.keys())
    totals = [stats[k]["tokens_in"] + stats[k]["tokens_out"] for k in keys]
    costs = [stats[k]["cost_usd"] for k in keys]
    x = list(range(len(keys)))
    width = 0.35

    fig, ax1 = plt.subplots(figsize=(6, 4.5))
    ax1.bar([i - width / 2 for i in x], totals, width, color="#3d5a80",
            label="career total tokens")
    ax1.set_ylabel("career total tokens")
    ax1.set_xticks(x)
    ax1.set_xticklabels(keys)

    ax2 = ax1.twinx()
    ax2.bar([i + width / 2 for i in x], costs, width, color="#e07a5f",
            label="career total cost ($)")
    ax2.set_ylabel("career total cost (USD)")

    ax1.set_title("Career token usage and cost by model")
    lines1, labels1 = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax1.legend(lines1 + lines2, labels1 + labels2, loc="upper right")
    fig.tight_layout()
    fig.savefig(out_path, dpi=150)
    plt.close(fig)
