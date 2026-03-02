#!/bin/bash
# Daily SQLite backup with rotation.
# Uses SQLite's .backup command which is safe with WAL mode.
# Keeps the last N backups (default 7).

set -euo pipefail

DB_PATH="${DB_PATH:-/home/exedev/feedreader/db.sqlite3}"
BACKUP_DIR="${BACKUP_DIR:-/home/exedev/feedreader/backups}"
KEEP_COUNT="${KEEP_COUNT:-7}"

# Ensure backup directory exists
mkdir -p "$BACKUP_DIR"

# Create timestamped backup using SQLite's .backup command
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_FILE="$BACKUP_DIR/db-$TIMESTAMP.sqlite3"

sqlite3 "$DB_PATH" ".backup '$BACKUP_FILE'"

# Compress the backup
gzip "$BACKUP_FILE"

echo "Backup created: ${BACKUP_FILE}.gz ($(du -sh "${BACKUP_FILE}.gz" | cut -f1))"

# Remove old backups, keeping only the most recent $KEEP_COUNT
cd "$BACKUP_DIR"
ls -t db-*.sqlite3.gz 2>/dev/null | tail -n +$((KEEP_COUNT + 1)) | xargs -r rm --

REMAINING=$(ls -1 db-*.sqlite3.gz 2>/dev/null | wc -l)
echo "Retained $REMAINING backup(s) in $BACKUP_DIR"
