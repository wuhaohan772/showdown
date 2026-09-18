#!/bin/sh
# Re-records docs/demo.cast and renders docs/demo.gif.
#
# The cast is generated, not captured: TestRecordDemo drives the real Model
# through a scripted match and writes each frame it draws. No agent CLI runs,
# so re-recording costs nothing and repeats exactly.
#
# Needs agg (https://github.com/asciinema/agg): winget install asciinema.agg,
# brew install agg, or cargo install --git https://github.com/asciinema/agg
set -e
cd "$(dirname "$0")/.."

SHOWDOWN_RECORD_DEMO=1 go test ./internal/tui/ -run TestRecordDemo -v

agg --theme github-dark --font-size 16 --fps-cap 20 --idle-time-limit 1 \
  docs/demo.cast docs/demo.gif

echo "wrote docs/demo.cast and docs/demo.gif"
