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
  - Spinner: animated indicator + current task name
  - Log viewport: scrollable live log (`↑/↓` to scroll)
  - Footer: status/hint line

### View Routing
- `withSidebar=false` → `cliView()` (minimal, no log)
- `withSidebar=true` → `execView()` (full panel with scrollable log)
- **Launcher** (`launcher.go:394`) passes `withSidebar=true`
- **Standalone runner** (`runner.go:92`) passes `withSidebar=false`

## Task Display & Logging

### Current Task Updates
- `currentTask` field updated every 50ms from `TASK [...]` lines in the log file
- Updated via `appendLine()` and `appendLines()` when parsing log lines
- Meta-tasks (`include_tasks`, `import_tasks`, `include_role`, `import_role`) are **skipped** — they don't update `currentTask`
- Task names are shortened via `shortTaskName()` function:
  - Strips role prefix: `"role : task"` → `"task"`
  - Extracts description after pipe: `"role : ... | description"` → `"description"`
  - Handles bare task names unchanged

### Log File Reading
- **Path:** `/usr/local/valet-sh/valet-sh/log/debug.log`
- **Rotation:** Ansible callback plugin rotates it at startup via `doRollover()`
- **Re-opened each tick** (not kept open) to detect rotation by inode
- **Inode tracking:** When inode changes, reset `logFileOffset` to 0 to read the new file from start
- **Seek-based offset:** Fresh `bufio.Reader` created each tick starting from `logFileOffset`
- This approach correctly detects:
  - New content appended to the file
  - File rotation (inode change)
  - EOF then growth (avoids `bufio.Reader` EOF caching issues)

## Error Handling in TUI Launcher

When a command fails:
1. `execDoneMsg` arrives with error
2. **First keypress** after failure shows the "View full log? [Y]/n" prompt
3. **Y or Enter** immediately opens the log viewer
4. **N or Esc** quits the program
5. Error handling is rendered in `execView()` footer when `e.done && e.err != nil`

This is different from CLI mode, which requires two keypresses (show prompt, then confirm).

## Code Locations

| Feature | File |
|---------|------|
| Inode tracking & file rotation detection | `exec.go:116-128` |
| Log file re-open with rotation handling | `exec.go:620-655` |
| Task text updates (skip meta-tasks) | `exec.go:543-576` |
| Task name shortening | `exec.go:851-875` |
| CLI mode (minimal view) | `exec.go:382-413` |
| TUI launcher mode (full view) | `exec.go:420-450` |
| Error prompt in TUI launcher | `exec.go:438-452` |
| Launcher configuration | `launcher.go:394` |
| Standalone runner configuration | `runner.go:92` |

## Testing

Run all tests with:
```bash
cd cli && go test ./internal/tui -v
```

Key test areas:
- Meta-task filtering
- Task name shortening
- Error prompt rendering
- Inode detection (manual testing recommended)
