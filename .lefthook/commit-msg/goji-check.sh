#!/bin/bash

# Get the commit message
INPUT_FILE=$1
COMMIT_MSG=$(head -n1 "$INPUT_FILE")

# Run Goji with the commit message as argument
RESULT=$(goji check --from-file "$COMMIT_MSG")

# Check if the result contains "Success:" using grep
if echo "$RESULT" | grep -q "Success:"; then
    exit 0
else
    echo "Commit message did not pass Goji check:"
    echo "$RESULT"
    exit 1
fi