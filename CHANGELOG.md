# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.18] - 2026-09-22

### Added & Improved
- **Active Account Email & Subscription Tier in `/usage`**:
  - Displays the active Google account email and subscription plan (e.g. `⭐ Langganan: Pro`, `💎 Ultra`, `🌟 Plus`, or `🆓 Free`) at the top of the `/usage` card.
  - Queries Google Cloud Code Assist (`loadCodeAssist`) API concurrently with `agy -p /usage` (zero latency impact) with 10-minute in-memory caching.
  - Enhanced `/whoami` and account profile details to display the active subscription tier badge.
  - Tier cache is automatically invalidated upon account switch or signout.

## [1.0.17] - 2026-09-21

### Added & Improved
- **Identifiable Media File Naming Format (`agy-bot-tt-bb-tahun_HHmmss`)**:
  - Replaced ambiguous generic Unix timestamp naming (`photo_<timestamp>.jpg`) with human-readable, bot-identifiable timestamp naming:
    - Photos: `agy-bot-DD-MM-YYYY_HHmmss.jpg` (e.g. `agy-bot-21-09-2026_191851.jpg`)
    - Documents: `agy-bot-DD-MM-YYYY_HHmmss_<filename>` (or `agy-bot-DD-MM-YYYY_HHmmss.bin` if unlabelled)
    - Video / Audio / Voice: prefixed with `agy-bot-DD-MM-YYYY_HHmmss_`
  - Prevents filename collision and makes it immediately clear in workspace and uploads directory which bot received the media.

## [1.0.16] - 2026-09-18

### Reverted
- **`/usage` back to bullet + progress-bar card (table experiment undone)**:
  - Per user feedback the swipeable monospace table was harder to read; restored the grouped per-scope bullet layout with status dots, progress bars, WIB reset times and reset countdowns from v1.0.15.

## [1.0.15] - 2026-09-18

### Improved
- **Beautified `/usage` quota card with reset countdowns**:
  - Raw TSV output (`Weekly Limit Remaining 79% 2026-09-24T16:01:16Z`) is now parsed and rendered as a one-glance summary plus a monospace table inside `<pre>`, so Telegram clients let users swipe sideways when it overflows the screen.
  - Table columns per scope row (♊ Gemini, Claude/GPT; 5-hour before weekly): remaining percent, reset countdown (`Reset dalam 6 hari 5 jam`, `segera (menunggu refresh)` when elapsed), and reset time in WIB — id/en localized.
  - Card carries in-place Refresh + Back buttons on both slash and button paths; falls back to the raw code block if the CLI output shape ever changes.

## [1.0.14] - 2026-09-18

### Fixed
- **Dynamic live busy card (queue status never looks frozen/stuck)**:
  - One live card per user is now edited in place instead of stacking static "please wait" messages: elapsed duration ticks every 30s, queue positions shift as items start, and the header transitions `✅ masuk antrean #N` → `▶️ Prompt antrean berhasil dijalankan` → `✅ Semua tugas selesai` / `🛑 Tugas dihentikan` automatically, with a `🔄 Status live • diperbarui HH:MM:SS` footer proving freshness.
  - `/clearqueue` and full-stop actions keep the card truthful (re-render or terminal state); tapping buttons on the card itself no longer gets overwritten by stale refreshes.
  - Fixed `/cancel` with a pending queue: the worker now re-registers the next queued turn with a fresh context, so cancellation stops only the current turn and the queue keeps auto-running (previously the continued turn ran unregistered, risking duplicate concurrent runs).

## [1.0.13] - 2026-09-18

### Fixed
- **Explicit FIFO message queue for long runs (no more ambiguous dropped messages)**:
  - Previously, any message sent while a task was running was silently discarded with only a "please wait" warning, so follow-ups like "lalu bisa ga ini di-add ke gogate" were lost and users had to resend manually.
  - Now every queueable prompt (plain text, `/plan`, `/goal`, `/continue`, unknown skill commands, media captions, artifact approve/reject) is either executed immediately when idle or explicitly enqueued with a position number (`✅ Pesan masuk antrean #N`) when busy — never silently dropped.
  - Queue drains automatically in order in the same worker (active task stays registered during drain, so no duplicate concurrent `agy` processes). Each queued start announces `▶️ Menjalankan antrean #N`.
  - New instant control commands: `/queue` (view FIFO card: active + pending) and `/clearqueue` (drop pending, keep active running). `/status` now always embeds the queue section. `/cancel` stops only the active turn and keeps the queue for auto-run; new `Hentikan Semua` button stops active + clears queue.
  - Split long-message debounce still works while busy: messages flow through the accumulator first, then the queue decision happens on the merged text, so multi-part pastes become a single queue entry.
  - Cap: max 10 pending per user, with explicit "queue full" notice instead of silent loss.

## [1.0.7] - 2026-09-16

### Added & Improved
- **100% Functional `/artifact` System (Open, Download, Approve, Reject)**:
  - Added dedicated `/artifact` and `/artifacts` slash commands to discover and review plans and artifacts generated by Antigravity.
  - Automatically queries active and recent conversation artifacts from `~/.gemini/antigravity-cli/brain/<conversation-id>/` with metadata parsing (`requestFeedback`, `summary`, file size, timestamps).
  - Interactive Inline Actions:
    - **Open (`art_open`)**: Reads artifact markdown and renders a formatted preview directly in Telegram chat.
    - **Download (`art_download`)**: Transmits the actual `.md` file as a native Telegram document to the user.
    - **Approve (`art_approve`)**: Confirms the plan and triggers Antigravity to proceed with implementation.
    - **Reject (`art_reject`)**: Signals rejection and instructs Antigravity to review and revise the plan.
- **Multimodal File & Media Processing (Photos, Documents, Videos, Audio)**:
  - Users can now send any photo, document, script, video, or voice recording to the Telegram bot with or without a caption.
  - Automatically downloads media to `<workspace>/uploads/<filename>` and provides an instant receipt badge.
  - Passes the downloaded file path and user caption directly to Antigravity CLI, allowing the agent to inspect the file using `view_file` or multimodal vision and stream the analysis back to the user.

## [1.0.6] - 2026-09-16

### Fixed & Improved
- **Continuous Live Typing Status (`typing...`)**:
  - Automatically broadcasts continuous Telegram `typing...` status every 4 seconds while Antigravity is processing, planning, calling tools, or generating output, providing immediate visual feedback in the Telegram header.
- **Resilient Non-Blocking Activity Tracker**:
  - Rewrote `ActivityTracker` into an actor-based architecture with an internal buffer channel and single throttled worker loop (800ms).
  - Eliminates lock contention and `time.Sleep` deadlocks during rapid tool calls (`replace_file_content`, `run_command`, etc.).
  - Added in-place completion fallback: transforms the activity message directly into the final result if no text deltas are streamed, completely preventing messages from being permanently stuck on `✏️ Edit(...)` badges.
- **Robust Subprocess Stream I/O without Size Limits**:
  - Replaced `bufio.Scanner` (which crashed with `bufio.ErrTooLong` on lines > 2MB and deadlocked the stdout pipe) with `bufio.Reader.ReadBytes('\n')` to seamlessly process arbitrary file diffs and tool output sizes.
- **Network Resilience & Connection Timeout**:
  - Equipped `BotAPI` with a custom `http.Client` featuring connection pooling and a 45-second timeout, preventing indefinite TCP hangs when internet disconnects and reconnects.

## [1.0.5] - 2026-09-15

### Added & Improved
- **Transparent Version Visibility**:
  - Displays explicit bot version (`v1.0.5`) in `/status` dashboard, `/help` menu, and `-version` CLI flag.
  - Automatically appends version badge (`🏷️ v1.0.5`) to every execution footer in Telegram responses so users can instantly verify the active running version.

## [1.0.4] - 2026-09-15

### Added & Improved
- **Auto-Deleting Real-Time Activity Badge**:
  - Dynamically displays real-time tool actions (e.g., `Bash(cmd)`, `View(path)`, `Edit(path)`, `Search(query)`) in a dedicated temporary status badge.
  - Automatically deletes the activity badge when the assistant begins outputting the final response or turn completes, ensuring zero chat clutter.
  - Guarantees the primary response message remains clean, permanent, and untouched.

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
