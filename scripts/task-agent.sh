#!/bin/bash
# Beads task agent — runs via systemd timer once per hour.
# Starts a shelley conversation with claude-opus-4.6 to pick ready
# tasks from beads and complete them.

set -euo pipefail

WORKDIR=/home/exedev/feedreader
TIMEOUT=3600  # 60 minutes
MODEL=claude-opus-4.6
LOGFILE=/home/exedev/feedreader/logs/task-agent.log

mkdir -p "$(dirname "$LOGFILE")"

log() {
    echo "[$(date -Iseconds)] $*" | tee -a "$LOGFILE"
}

log "Starting task agent"

# Check if there is ready work
READY=$(cd "$WORKDIR" && bd ready --json 2>/dev/null || echo '[]')
COUNT=$(echo "$READY" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo 0)

if [ "$COUNT" -eq 0 ]; then
    log "No ready tasks. Nothing to do."
    exit 0
fi

log "Found $COUNT ready task(s)"

PROMPT='You are an autonomous task agent. Your job is to pick ready tasks from beads and complete them.

STEPS:

1. Run `bd ready` to see available work. If nothing is ready, say "No ready tasks" and stop.

2. Pick 1-2 tasks. Prefer higher priority. Prefer tasks that are logically related.

3. Claim each task: `bd update <id> --claim`

4. Complete each task:
   - Make the code changes
   - Ensure existing functionality is preserved (no regressions)
   - Keep the app in a working state at all times

5. Run `make check` after completing your changes. If it fails, attempt to fix the issues. Iterate until it passes or you are confident you cannot fix it (in which case, revert your changes for that task and note the issue as a comment on the bead).

6. Close completed tasks: `bd close <id> --reason "description of what was done"`

7. If you discover new work while completing a task, create new beads:
   `bd create "title" --description="..." --type=task|bug|feature`

8. Commit all code changes with a clear commit message.

IMPORTANT RULES:
- Do NOT attempt to do everything at once. Pick 1-2 tasks only.
- Always run `make check` before committing. All checks must pass.
- Never break existing app functionality. The app must work after your changes.
- If you get stuck on a task, add a comment with `bd comments add <id> "stuck: reason"`, unclaim it, and move on.
- Do not modify this prompt or the service/timer configuration.'

# Start the conversation
RESULT=$(shelley client chat -p "$PROMPT" -model "$MODEL" -cwd "$WORKDIR" 2>&1)
CONV_ID=$(echo "$RESULT" | jq -r '.conversation_id // empty' 2>/dev/null || true)

if [ -z "$CONV_ID" ]; then
    log "ERROR: Failed to start conversation. Output: $RESULT"
    exit 1
fi

log "Started conversation: $CONV_ID"

# Wait for completion with timeout
timeout "$TIMEOUT" shelley client read -wait "$CONV_ID" >> "$LOGFILE" 2>&1 || {
    EXIT_CODE=$?
    if [ $EXIT_CODE -eq 124 ]; then
        log "ERROR: Agent timed out after ${TIMEOUT}s"
    else
        log "ERROR: Agent read failed with exit code $EXIT_CODE"
    fi
    exit $EXIT_CODE
}

log "Agent finished successfully"

# Archive the conversation to keep things tidy
shelley client archive "$CONV_ID" >> "$LOGFILE" 2>&1 || true

log "Done"
