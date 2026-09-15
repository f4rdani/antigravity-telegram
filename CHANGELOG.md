# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
