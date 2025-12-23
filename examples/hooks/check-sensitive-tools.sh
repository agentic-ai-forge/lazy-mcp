#!/bin/bash
#
# Claude Code PreToolUse Hook for lazy-mcp
#
# This hook inspects execute_tool calls and decides whether to:
#   - Allow silently (exit 0, no output)
#   - Ask for permission (exit 0 with JSON permissionDecision: "ask")
#   - Deny (exit 0 with JSON permissionDecision: "deny")
#
# Configuration via environment variables:
#   LAZY_MCP_SENSITIVE_TOOLS - Comma-separated list of tool patterns requiring permission
#   LAZY_MCP_DENIED_TOOLS    - Comma-separated list of tool patterns to block entirely
#
# Tool patterns support:
#   - Exact match: "gmail.send_email"
#   - Server prefix: "gmail.*" (all gmail tools)
#   - Suffix match: "*.delete_*" (all delete operations)
#
# Example:
#   export LAZY_MCP_SENSITIVE_TOOLS="gmail.send_email,gmail.create_draft,github.create_*,gitlab.create_*"
#   export LAZY_MCP_DENIED_TOOLS="gmail.delete_email"
#

set -euo pipefail

# Default sensitive tools (public-facing actions)
DEFAULT_SENSITIVE_TOOLS=(
    # Gmail - sending/modifying emails
    "gmail.send_email"
    "gmail.create_draft"
    "gmail.delete_email"
    "gmail.trash_email"

    # GitHub - public repository actions
    "github.create_issue"
    "github.create_pull_request"
    "github.create_repository"
    "github.create_or_update_file"
    "github.push_files"
    "github.create_branch"
    "github.fork_repository"

    # GitLab - public repository actions
    "gitlab.create_issue"
    "gitlab.create_merge_request"
    "gitlab.accept_merge_request"
    "gitlab.create_project"
    "gitlab.create_branch"
    "gitlab.create_or_update_file"
)

# Default denied tools (dangerous operations)
DEFAULT_DENIED_TOOLS=(
    # Add patterns for tools that should never be allowed
    # Example: "*.force_delete_*"
)

# Parse environment variables into arrays
IFS=',' read -ra ENV_SENSITIVE_TOOLS <<< "${LAZY_MCP_SENSITIVE_TOOLS:-}"
IFS=',' read -ra ENV_DENIED_TOOLS <<< "${LAZY_MCP_DENIED_TOOLS:-}"

# Combine defaults with environment overrides
SENSITIVE_TOOLS=("${DEFAULT_SENSITIVE_TOOLS[@]}" "${ENV_SENSITIVE_TOOLS[@]}")
DENIED_TOOLS=("${DEFAULT_DENIED_TOOLS[@]}" "${ENV_DENIED_TOOLS[@]}")

# Read input from stdin (Claude Code passes JSON)
INPUT=$(cat)

# Extract tool_path from the arguments
# Input format: {"tool_name": "mcp__lazy-mcp__execute_tool", "tool_input": {"tool_path": "...", "arguments": {...}}}
TOOL_PATH=$(echo "$INPUT" | jq -r '.tool_input.tool_path // empty' 2>/dev/null)

if [[ -z "$TOOL_PATH" ]]; then
    # Not an execute_tool call or malformed input, allow by default
    exit 0
fi

# Pattern matching function
matches_pattern() {
    local tool="$1"
    local pattern="$2"

    # Handle wildcards
    if [[ "$pattern" == *"*"* ]]; then
        # Convert glob pattern to regex
        local regex="${pattern//\./\\.}"  # Escape dots
        regex="${regex//\*/.*}"           # Convert * to .*
        regex="^${regex}$"                # Anchor

        if [[ "$tool" =~ $regex ]]; then
            return 0
        fi
    else
        # Exact match
        if [[ "$tool" == "$pattern" ]]; then
            return 0
        fi
    fi

    return 1
}

# Output JSON for permission decision (must go to stdout, exit 0)
output_decision() {
    local decision="$1"
    local reason="$2"
    cat <<EOF
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "$decision",
    "permissionDecisionReason": "$reason"
  }
}
EOF
    exit 0
}

# Check against denied patterns first
for pattern in "${DENIED_TOOLS[@]}"; do
    [[ -z "$pattern" ]] && continue
    if matches_pattern "$TOOL_PATH" "$pattern"; then
        output_decision "deny" "Tool $TOOL_PATH is blocked by security policy"
    fi
done

# Check against sensitive patterns
for pattern in "${SENSITIVE_TOOLS[@]}"; do
    [[ -z "$pattern" ]] && continue
    if matches_pattern "$TOOL_PATH" "$pattern"; then
        output_decision "ask" "Tool $TOOL_PATH is a public-facing action and requires confirmation"
    fi
done

# Not in any list - allow silently (no output = allow)
exit 0
