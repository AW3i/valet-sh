# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### CLI Task Display & Log Viewer Improvements

#### Changed

**Task Display: Queue Model → Hold-Timer Model** ([`49c1675`](https://github.com/valet-sh/valet-sh/commit/49c1675))

Redesigned the CLI execution panel task display from a queue-based model to a hold-timer model to fix freezing during long-running Ansible tasks.

**Problem:** The queue model buffered discovered tasks and drained them at a fixed pace (one per tick, then one per 5 ticks). This caused:
- **Queue exhaustion**: All tasks from the first ~300ms of log output were consumed within 2-3 seconds
- **Display freeze**: Once the queue emptied, the task name stayed frozen at the last dequeued task (e.g., "ensure rabbitmq is started") while Ansible continued working on that task for 30-60+ seconds, giving no feedback

**Solution:** Replace queue with a simpler model that shows what Ansible is currently doing:
- `nextTask string` — most recently discovered task, waiting to be shown
- `lastTaskChangedAt time.Time` — timestamp of when `currentTask` was last updated
- Each discovered task displays for minimum 250ms before advancing to the next pending task
- Only one "pending" task is kept in memory at a time (no queue backlog)

**Benefits:**
- No queue exhaustion — only one pending task kept in buffer
- Always accurate — displays what Ansible is currently doing, not historical replay
- Natural pacing — fast tasks each get brief visibility; slow long-running tasks naturally stay showing longer
- Clean implementation — simpler logic, fewer edge cases

**Code changes:**
- `ExecModel` struct: removed `taskQueue []string`, `taskQueueTick int`; added `nextTask string`, `lastTaskChangedAt time.Time`
- `appendLine()` / `appendLines()` — set `nextTask` to discovered task name instead of appending to queue
- `execTickMsg` handler — use `time.Since(lastTaskChangedAt)` to advance task when 250ms+ elapsed
- `execDoneMsg` handler — flush `nextTask` to `currentTask` on completion

See [docs/architecture.md - CLI Task Display](docs/architecture.md#cli-task-display-hold-timer-model) for detailed design documentation.

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
- `49c1675` — Replace task queue with hold-timer mechanism and fix single-y log open (**current**)

**Files modified:**
- `cli/internal/tui/exec.go` — core task display logic, key handling
- `docs/architecture.md` — design documentation

**Testing:**
- Build: successful, no compilation errors
- Binary verified: `dist/valet` created at `2.9.19-135-g3e42896-dirty`

---

## [2.9.19] - Previous Release

(Earlier changes documented in git log)
