# valet-sh Development Guidelines

## Build After Every Code Change

**CRITICAL:** After every code change, always build `dist/valet`:

```bash
cd ../valet-sh-cli && make build
```

The Go CLI source has moved to the sibling `valet-sh-cli` repository.
The developer tests directly from `valet-sh-cli/dist/valet`. Failing to
build means the developer is testing stale code. Never skip this step.

**Also set `VALET_REPO_DIR`** so the dev binary reads playbooks and
`ansible.cfg` from this repo instead of the installed production path:

```bash
export VALET_REPO_DIR=/path/to/valet-sh
```

Without this, the binary uses `/usr/local/valet-sh/valet-sh` which may have
a stale `ansible.cfg` and callback plugin — causing silent failures.

## Binary Locations & Entry Points

- **`/usr/local/bin/valet.sh`** — Bash wrapper script. Does NOT use the Go binary directly; delegates via `exec` to the installed Go binary.
- **`../valet-sh-cli/dist/valet`** — Go binary (dev build). Developer tests directly from this path.
- **`/usr/local/valet-sh/bin/valet`** — Installed copy of the Go binary. Created by `make install` in `valet-sh-cli/`.

## TUI Architecture

### View Modes
- **CLI mode** (`dist/valet init-instance`): Minimal 2-line view
  - Header: command + version
  - Spinner: animated indicator + current task name
  - No log panel, no scrolling
  
- **TUI launcher mode** (`dist/valet` with no args, navigate menu → Enter): Full panel view
  - Header: breadcrumb + version
  - Spinner: animated indicator + current task name (updated in real time from JSON events)
  - Footer: status/hint line with keybinding hints
  - **On failure:** exits BubbleTea, displays full log to stdout with native terminal selection (Kitty-compatible)

### View Routing
- **CLI mode** — `runner.go:RunSubprocess()` → `runExecPanel()` starts BubbleTea with `standaloneExecModel`
  - Displays `cliView()` (minimal progress bar)
  - No sidebar, no scrolling
- **Launcher mode** — `launcher.go` manages screen state machine
  - `screenList` (command list with filter)
  - `screenInline` (argument input + help preview)
  - `screenExec` (execution panel with spinner)
  - `screenHelp` (read-only scrollable help viewer)
  - **On failure:** calls `printLogView()` which exits BubbleTea and prints log to stdout

## Task Display & Logging

### Current Task Updates
- `currentTask` field updated in real time as Ansible events arrive via JSON streaming
- Updated via `parseJSONEvent()` when `v2_playbook_on_task_start` events are received
- Meta-tasks (`include_tasks`, `import_tasks`, `include_role`, `import_role`) are **skipped** — they don't update `currentTask`
- Task names are shortened via `shortTaskName()` function:
  - Strips role prefix: `"role : task"` → `"task"`
  - Extracts description after pipe: `"role : ... | description"` → `"description"`
  - Handles bare task names unchanged

### JSON Event Streaming
- **Callback:** `ansible.posix.jsonl` (official Ansible callback, not custom Python)
- **Format:** One JSON object per line to stdout; contains structured task name, result data, and error details
- **Consumption:** `readTaskCmd()` goroutine reads pipe in real time, parses JSON lines via `parseJSONEvent()`
- **Error Details:** Structured JSON result includes `msg`, `stderr`, `stdout`, `rc`, `cmd` fields
  - `formatFailureLines()` and `formatWarningLines()` compose readable error blocks
  - Developers see failure context immediately in TUI without hunting through log files
- **EOF Detection:** When pipe closes, `readTaskCmd()` sends `ansibleEventMsg{eof: true}` to coordinate quit with process exit

## Error Handling in TUI Launcher

When a command fails:
1. `execDoneMsg` arrives with error
2. **First keypress** after failure shows the "View full log? [Y]/n" prompt
3. **Y or Enter** immediately opens the log viewer
4. **N or Esc** quits the program
5. Error handling is rendered in `execView()` footer when `e.done && e.err != nil`

This is different from CLI mode, which requires two keypresses (show prompt, then confirm). The failure context (stderr, return code, command that failed) comes from structured JSON event results, not file-based logging.

## Code Locations

| Feature | File |
|---------|------|
| Ansible JSON schema & parsing | `exec.go:10-80` (imports) + `ansible_events.go:1-60` |
| Task text updates (skip meta-tasks) | `ansible_events.go:180-220` |
| Task name shortening | `ansible_events.go:240-270` |
| Error formatting (failure/warning blocks) | `ansible_events.go:90-180` |
| CLI mode (minimal view) | `exec.go:382-413` |
| TUI launcher mode (full view) | `exec.go:420-450` |
| Error prompt in TUI launcher | `exec.go:438-452` |
| Launcher configuration & screen routing | `launcher.go:1-100` |
| Standalone runner configuration | `runner.go:70-150` |
| Interactive help viewer | `help.go:1-150` |
| Terminal layout utilities | `layout.go:1-60` |

## Testing

Run all tests with:
```bash
cd ../valet-sh-cli && go test ./internal/tui -v
```

Key test areas:
- Meta-task filtering
- Task name shortening
- Error prompt rendering
- JSON event parsing and error formatting
- Help view state transitions
- Screen state machine routing
