#!/bin/bash
# ES Modules refactor agent — runs via systemd timer once per hour.
# Starts a shelley conversation with claude-opus-4.6 to pick 2-3 tasks
# from docs/ES_MODULES_TODO.md and complete them.

set -euo pipefail

WORKDIR=/home/exedev/feedreader
TIMEOUT=3600  # 60 minutes
MODEL=claude-opus-4.6
LOGFILE=/home/exedev/feedreader/logs/es-modules-refactor.log

mkdir -p "$(dirname "$LOGFILE")"

log() {
    echo "[$(date -Iseconds)] $*" | tee -a "$LOGFILE"
}

log "Starting ES modules refactor agent"

# Check if all tasks are already done
if ! grep -q '^- \[ \]' "$WORKDIR/docs/ES_MODULES_TODO.md" 2>/dev/null; then
    log "All tasks completed. Nothing to do."
    exit 0
fi

PROMPT='You are an autonomous refactoring agent. Your job is to incrementally migrate srv/static/app.js to ES modules.

STEPS:

1. Read these files thoroughly:
   - docs/ES_MODULES_PLAN.md (the overall strategy)
   - docs/ES_MODULES_TODO.md (the task checklist)
   - docs/MEMORY.md (notes from previous runs)

2. If there are no unchecked tasks ("- [ ]") in ES_MODULES_TODO.md, say "All tasks completed" and stop.

3. Pick 2-3 unchecked tasks from ES_MODULES_TODO.md. Choose tasks that are sequential and logically related. Respect phase ordering — finish Phase 1 before starting Phase 2, etc. If a previous run left notes in MEMORY.md about what to do next, follow those.

4. Complete each task:
   - Make the code changes
   - Ensure existing functionality is preserved (no regressions)
   - Keep the app in a working state at all times

5. Run `make check` after completing your changes. If it fails, attempt to fix the issues. Iterate until it passes or you are confident you cannot fix it (in which case, revert your changes for that task and note the issue in MEMORY.md).

6. Update docs/ES_MODULES_TODO.md — check off completed tasks by changing "- [ ]" to "- [x]".

7. Update docs/MEMORY.md with:
   - What you completed this run
   - Any issues encountered and how you resolved them
   - Context the next run should know (e.g., partially done work, dependencies, gotchas)
   - Keep it concise — this is working memory, not a journal

8. Commit all changes with a clear commit message describing what was refactored.

IMPORTANT RULES:
- Do NOT attempt to do everything at once. Pick 2-3 tasks only.
- Always run `make check` before committing. All checks must pass.
- Never break existing app functionality. The app must work after your changes.
- If you get stuck on a task, skip it, note why in MEMORY.md, and move on.
- Do not modify this prompt or the service/timer configuration.'
- When testing in browser, use the dev user.

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
