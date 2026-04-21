#!/usr/bin/env bash
# check-lane.sh — verify a subagent only modified files in their owned directories.
#
# Usage: ./scripts/check-lane.sh <lane-name>
# Example: ./scripts/check-lane.sh L3
#
# Exits 0 if all modifications are within the lane's owned paths, non-zero otherwise.

set -euo pipefail

LANE="${1:-}"
if [ -z "$LANE" ]; then
    echo "Usage: $0 <lane-name>" >&2
    echo "Lanes: L1, L2, L3, L4, L5, L6, L7, L8" >&2
    exit 2
fi

# Lane ownership — must match LANES.md
declare -A ALLOWED_PATHS
ALLOWED_PATHS[L1]="^(internal/runtime/k8s/|internal/scheduler/|images/demo-agent/|deploy/examples/hello-skill\.yaml|go\.mod|go\.sum)"
ALLOWED_PATHS[L2]="^(internal/runtime/local/|internal/runtime/docker/|go\.mod|go\.sum)"
ALLOWED_PATHS[L3]="^(internal/policy/|deploy/examples/policy\.yaml|go\.mod|go\.sum)"
ALLOWED_PATHS[L4]="^(internal/storage/postgres/|go\.mod|go\.sum)"
ALLOWED_PATHS[L5]="^(internal/gateway/slack/|go\.mod|go\.sum)"
ALLOWED_PATHS[L6]="^(web/)"
ALLOWED_PATHS[L7]="^(deploy/helm/|\.github/workflows/|go\.mod|go\.sum)"
ALLOWED_PATHS[L8]="^(README\.md|docs/)"

ALLOWED="${ALLOWED_PATHS[$LANE]:-}"
if [ -z "$ALLOWED" ]; then
    echo "ERROR: unknown lane '$LANE'" >&2
    exit 2
fi

# Get changed files relative to merge-base with master
BASE=$(git merge-base HEAD master 2>/dev/null || git rev-list --max-parents=0 HEAD)
CHANGED=$(git diff --name-only "$BASE" HEAD)

if [ -z "$CHANGED" ]; then
    echo "No changes detected."
    exit 0
fi

VIOLATIONS=0
echo "Lane $LANE — allowed paths: $ALLOWED"
echo ""
while IFS= read -r file; do
    if [[ ! "$file" =~ $ALLOWED ]]; then
        echo "  [VIOLATION] $file"
        VIOLATIONS=$((VIOLATIONS + 1))
    else
        echo "  [OK]        $file"
    fi
done <<< "$CHANGED"

echo ""
if [ "$VIOLATIONS" -gt 0 ]; then
    echo "FAIL: $VIOLATIONS files outside lane boundaries"
    exit 1
fi
echo "PASS: all changes within lane boundaries"
