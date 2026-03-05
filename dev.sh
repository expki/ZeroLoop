#!/usr/bin/env bash
set -e

SESSION="zeroloop"
DIR="$(cd "$(dirname "$0")" && pwd)"

# Kill existing session if present
tmux kill-session -t "$SESSION" 2>/dev/null || true

# Create session with backend window
tmux new-session -d -s "$SESSION" -n backend -c "$DIR"
tmux send-keys -t "$SESSION:backend" "ENVIRONMENT=development PORT=9368 go run ." Enter

# Create frontend window
tmux new-window -t "$SESSION" -n frontend -c "$DIR/ui"
tmux send-keys -t "$SESSION:frontend" "npm run dev" Enter

# Enable mouse for clickable tabs
tmux set-option -t "$SESSION" -g mouse on

# Attach
tmux attach-session -t "$SESSION"
