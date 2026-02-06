#!/bin/bash

# Script to clear all data from database tables

DB_PATH="./data/jobs.db"

if [ ! -f "$DB_PATH" ]; then
    echo "Database file not found at $DB_PATH"
    exit 1
fi

echo "Clearing data from database tables..."

# Clear jobs table
sqlite3 "$DB_PATH" "DELETE FROM jobs;"

# Clear dead letter queue table
sqlite3 "$DB_PATH" "DELETE FROM dead_letter_jobs;"

# Reset SQLite sequence (if using AUTOINCREMENT, though we're not)
# sqlite3 "$DB_PATH" "DELETE FROM sqlite_sequence;"

echo "Database cleared successfully!"
echo ""
echo "Verifying..."
sqlite3 "$DB_PATH" "SELECT COUNT(*) as jobs_count FROM jobs; SELECT COUNT(*) as dlq_count FROM dead_letter_jobs;"
