# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.3] - 2026-09-15

### Added & Fixed
- **Transparent Real-Time Activity Tracking**:
  - Automatically captures `step_update` tool events (`run_command`, `view_file`, `write_to_file`, `replace_file_content`, `grep_search`, `find_by_name`, web tools, etc.) and thinking steps.
  - Displays live, human-readable action badges and recent step breadcrumbs during agent execution.
  - Updates in-place with zero chat clutter; smoothly transitions to streaming assistant response and final clean answer upon completion.
- **Fixed `(Empty response)` Bug**:
  - Replaced ambiguous `(Empty response)` fallback when agent turns complete after running background tools without emitting text deltas.
  - Automatically formats a clean summary of completed actions.

## [1.0.2] - 2026-09-15

### Fixed & Improved
- **Human-Readable Conversation Titles in `/resume`**:
  - Directly reads `~/.gemini/antigravity-cli/conversation_summaries.db` using pure-Go SQLite (`modernc.org/sqlite`, CGO-free) to retrieve actual conversation topics (`preview`) instead of raw UUIDs or unwanted code blocks.
  - Displays relative timestamps (e.g., `Baru saja`, `15 mnt lalu`, `Kemarin`) and workspace path on each session button.
  - Resuming binds the session cleanly and lets the user type their next message without triggering unsolicited agent turns.
  - Supports `/resume <id>` and `/switch <id>` with short ID prefix matching, alongside interactive inline keyboard buttons.
- **Pure-Go Cross-Compilation**:
  - Full compatibility with `CGO_ENABLED=0` across Linux (`amd64`/`arm64`), Windows, and macOS without external C toolchains.

## [1.0.1] - 2026-09-15

### Added
- **Interactive `/resume` Conversation Picker**:
  - Automatically lists recent conversations from both bot session storage and `~/.gemini/antigravity-cli/history.jsonl`.
  - Interactive Inline Keyboard buttons allowing users to tap and resume any past conversation ID directly.
  - Automatic workspace directory alignment when resuming conversations.
  - Quick action button `[ 💬 Lanjut Percakapan Terakhir ]` to immediately resume from mobile.
- **Automatic Telegram Slash Command Menu (`setMyCommands`)**:
  - Automatically registers all bot slash commands with the Telegram Bot API on startup so typing `/` in Telegram clients (Mobile, Desktop, Web) immediately displays the command popup autocomplete list with descriptions.

## [1.0.0] - 2026-09-15

### Added
- **Dual-Engine Execution Architecture**:
  - **Mode A (Fast CLI Inspection)**: Instant execution for `/usage`, `/credits`, `/skills`, `/model`, `/effort`, `/agents`, and `/changelog`.
  - **Mode B (Stream-JSON Agent Engine)**: Multi-turn prompt coding, `/plan <task>`, `/goal <task>`, and skills execution using newline-delimited JSON (`NDJSON`).
- **Zero Chat Clutter & In-Place UI**:
  - Interactive menus edit in-place without flooding chat history.
  - Instant dismissal buttons (`[ 🗑️ Tutup ]`) for temporary configuration and info cards.
  - In-place refresh buttons (`[ 🔄 Refresh ]`) for live quota and status tracking.
- **Throttled Live Streaming**:
  - Rate-limited message editing (1200ms buffer) to stream agent thinking and token deltas smoothly without triggering Telegram `429 Too Many Requests`.
  - Automatic fallback protection from HTML parse errors to plain text.
- **Auto-Sanitization of File Protocol Links**:
  - Automatically transforms internal Antigravity `[`path`](file:///path)` links into clean, beautiful inline code badges (`<code>path</code>`).
- **User-Configurable Tool Permissions**:
  - Toggle between `auto` (`--dangerously-skip-permissions` for hands-free autonomous workflows) and `ask` (manual confirmation).
- **Workspace & Server Navigation**:
  - Default workspace dynamically resolves to filesystem Root (`/` on Linux, `C:\` on Windows).
  - Dynamic directory switching and inspection via `/cwd <path>`, `/pwd`, `/ls [path]`, and `/file <path>`.
- **Security Whitelist**:
  - Single-tenant security model enforcing Telegram User ID whitelist (`allowed_user_ids`).
- **Cross-Platform Background Daemon**:
  - Statically compiled standalone Go binary (< 10MB) requiring ≤ 15MB RAM idle.
  - Ready-to-use `systemd` service configuration for headless Linux server deployment.
  - Automated GitHub Actions workflow for cross-compiling Linux (amd64/arm64), Windows (amd64), and macOS (amd64/arm64).
