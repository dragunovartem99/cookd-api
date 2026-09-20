#!/usr/bin/env bash
# Fails if any .go file has more than MAX lines, not counting blank lines and comments.
set -euo pipefail

MAX=${MAX:-100}
status=0

while IFS= read -r file; do
	count=$(awk '
		/^[[:space:]]*$/ { next }
		incomment { if (/\*\//) incomment = 0; next }
		/^[[:space:]]*\/\// { next }
		/^[[:space:]]*\/\*/ { if (!/\*\//) incomment = 1; next }
		{ n++ }
		END { print n + 0 }
	' "$file")
	if [ "$count" -gt "$MAX" ]; then
		echo "$file: $count lines (max $MAX)"
		status=1
	fi
done < <(find . -name '*.go' -not -path './.git/*')

exit $status
