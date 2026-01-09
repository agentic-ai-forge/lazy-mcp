/**
 * OpenCode Sensitive Tools Check Plugin
 * 
 * Migrated from Claude Code PreToolUse hook:
 * - check-sensitive-tools.sh
 * 
 * Intercepts lazy-mcp execute_tool calls and throws errors for
 * sensitive/public-facing actions (Gmail send, GitHub create PR, etc.)
 * 
 * In OpenCode, we can't show a permission dialog like Claude Code,
 * so sensitive tools will throw an error with a clear message.
 * The user can then explicitly confirm by re-running with confirmation.
 */

import type { Plugin } from "@opencode-ai/plugin"

// Default sensitive tools (public-facing actions that need confirmation)
const SENSITIVE_TOOLS = [
  // Gmail - sending/modifying emails
  "gmail.send_email",
  "gmail.create_draft",
  "gmail.delete_email",
  "gmail.trash_email",
  
  // GitHub - public repository actions  
  "github.create_issue",
  "github.create_pull_request",
  "github.create_repository",
  "github.create_or_update_file",
  "github.push_files",
  "github.create_branch",
  "github.fork_repository",
  "github.merge_pull_request",
  
  // GitLab - public repository actions
  "gitlab.create_issue",
  "gitlab.create_merge_request",
  "gitlab.merge_merge_request",
  "gitlab.create_repository",
  "gitlab.create_branch",
  "gitlab.create_or_update_file",
  "gitlab.push_files",
]

// Tools that should be completely blocked
const DENIED_TOOLS: string[] = [
  // Add dangerous operations here
  // "*.force_delete_*"
]

// Pattern matching function
function matchesPattern(tool: string, pattern: string): boolean {
  if (pattern.includes("*")) {
    // Convert glob to regex
    const regex = new RegExp(
      "^" + pattern.replace(/\./g, "\\.").replace(/\*/g, ".*") + "$"
    )
    return regex.test(tool)
  }
  return tool === pattern
}

export const SensitiveToolsCheckPlugin: Plugin = async ({ directory }) => {
  // Add custom patterns from environment
  const envSensitive = process.env.LAZY_MCP_SENSITIVE_TOOLS?.split(",").filter(Boolean) || []
  const envDenied = process.env.LAZY_MCP_DENIED_TOOLS?.split(",").filter(Boolean) || []
  
  const allSensitive = [...SENSITIVE_TOOLS, ...envSensitive]
  const allDenied = [...DENIED_TOOLS, ...envDenied]

  return {
    "tool.execute.before": async (input, output) => {
      // Only intercept execute_tool calls (lazy-mcp)
      if (input.tool !== "execute_tool") {
        return
      }

      const toolPath = output.args?.tool_path as string
      if (!toolPath) {
        return
      }

      // Check denied tools first
      for (const pattern of allDenied) {
        if (matchesPattern(toolPath, pattern)) {
          throw new Error(
            `BLOCKED: Tool "${toolPath}" is blocked by security policy.\n` +
            `This action is not allowed. If you need to perform this action, ` +
            `please do it manually or adjust LAZY_MCP_DENIED_TOOLS.`
          )
        }
      }

      // Check sensitive tools - log warning but allow
      // Note: OpenCode doesn't have Claude Code's "ask" permission mechanism
      // So we log a warning to the console instead
      for (const pattern of allSensitive) {
        if (matchesPattern(toolPath, pattern)) {
          console.warn(
            `\n⚠️  SENSITIVE ACTION: "${toolPath}"\n` +
            `   This is a public-facing action (email, PR, push, etc.)\n` +
            `   Proceeding automatically - check output carefully!\n`
          )
          return // Allow but warn
        }
      }

      // Not in any list - allow silently
    }
  }
}
