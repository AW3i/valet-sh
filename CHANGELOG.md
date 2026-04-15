# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### CLI Task Display & Log Viewer Improvements

#### Changed

**Task Display: Hold-Timer Model → Direct-Assignment Model (Simpler, Correct Behavior)** 

The hold-timer model attempted to show the most recent task while buffering pending ones, but had a critical initialization bug that caused the display to freeze. Reverted to a simpler, more correct model that directly assigns `currentTask` to the most recently discovered task.

**Previous Problem (Hold-Timer):** 
- During the initial 300ms, ~80 tasks arrived in the first log batch. The init guard would fire and lock `nextTask` to empty, leaving the display showing only the first task while Ansible executed tasks 2-80 and beyond
- During long operations (RabbitMQ wait, 60+ seconds): no new `TASK [...]` lines meant `nextTask` never updated, so the display stayed frozen on whatever task was displayed when the long operation started
- Root cause: trying to animate/queue historical tasks instead of directly reflecting what Ansible is currently executing

**Solution:** Simplify to always show the most recently discovered task:
- Remove `nextTask` and `lastTaskChangedAt` fields — no queuing or state machine needed
- In `appendLine()` / `appendLines()`: directly set `currentTask = taskName` whenever a `TASK [...]` line is discovered
- Result: display always shows what Ansible is currently on, naturally updates as new tasks begin

**Why This Works:**
- When 80 tasks run in 300ms: `currentTask` updates 80 times, ends at task 81
- When task 81 is blocked for 60 seconds: no new `TASK [...]` lines, `currentTask` stays at task 81 (the one actually running)
- No disconnection from reality, no freezing, no complex state machines

**Code changes:**
- `ExecModel` struct: removed `nextTask string`, `lastTaskChangedAt time.Time`
- `appendLine()` / `appendLines()` — simplified to directly assign `currentTask = taskName`
- `execTickMsg` handler — removed hold-timer logic entirely
- `execDoneMsg` handler — removed `nextTask` flush logic

**Historical iterations:**
- `517fd52` — Initial queue model (exhaustion problem)
- `3e42896` — Paced queue (still had exhaustion during long ops)
- `49c1675` — Hold-timer model (initialization and freezing bug)
- Latest — Revert to direct-assignment (correct, simplest design)

See [docs/architecture.md - CLI Task Display](docs/architecture.md#cli-task-display-direct-assignment-model) for detailed design documentation.

---

**Log Viewer Prompt: Skip Intermediate State on Y/Y** ([`49c1675`](https://github.com/valet-sh/valet-sh/commit/49c1675))

Fixed the "View full log?" prompt interaction to open log viewer with a single keypress.

**Problem:** When Ansible failed:
1. User presses any key → prompt appears "View full log? [Y/n]"
2. User presses Y → log viewer opens
- Result: required two separate keypresses to view the log

**Solution:** When the first error-state keypress is `y` or `Y`, immediately open the log viewer without showing the intermediate prompt. Other keys still show the prompt as expected.

**Code changes:**
- `handleKey()` in error state — added check: if key is `y/Y`, call `loadLogCmd()` directly; otherwise show prompt

**User experience:**
- Pressing `y` directly on error: log opens immediately (one keystroke)
- Pressing other key on error: prompt shows, user can then press `y/Y` or `n` (two stage process)
- Consistent with natural user expectation: "I want to see the log" → single action

---

### Technical Details

**Commits:**
- `517fd52` — Implement task queue for smooth sequential task display in CLI (superseded)
- `3e42896` — Fix task queue pacing and Y key handling (superseded)
- `49c1675` — Hold-timer model (reverted due to initialization bug)
- Latest — Revert to direct-assignment model, simpler and correct (**current**)

**Files modified:**
- `cli/internal/tui/exec.go` — simplified task display logic (removed queue/hold-timer)
- `docs/architecture.md` — updated design documentation

**Testing:**
- Build: successful, no compilation errors
- Changes are in `cli` subdirectory with its own `go.mod`

---

## [2.9.19] - Previous Release

(Earlier changes documented in git log)
