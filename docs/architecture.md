# valet-sh Architecture

## System Overview

How the four repositories work together to deliver a working installation on a developer machine.

```mermaid
graph TD
    User["👤 Developer"]

    subgraph Install["Installation"]
        InstallSH["valet-sh/install\ncurl | bash one-liner"]
        Installer["valet-sh/installer\nGo binary\nsetup · update · download CLI"]
        Runtime["valet-sh/runtime\nPython venv tarball\nAnsible + pip packages"]
        CLIPkg["valet-sh/cli\nPython package\nbash wrapper (valet.sh)"]
    end

    subgraph ThisRepo["valet-sh/valet-sh  (this repo)"]
        GoCLI["Go CLI binary\n/usr/local/valet-sh/bin/valet"]
        Playbooks["Ansible Playbooks\nplaybooks/ · roles/"]
    end

    subgraph DiskLayout["/usr/local/valet-sh/"]
        BinDir["bin/valet  ← Go CLI"]
        VenvDir["venv/  ← Python + Ansible\nvenv/bin/valet.sh  ← bash wrapper"]
        EtcDir["etc/  ← config.yml · links.yml"]
        LogDir["valet-sh/log/debug.log"]
    end

    subgraph Services["Managed Services (host OS)"]
        PHP["PHP-FPM\n5.6 – 8.5"]
        DB["MariaDB / MySQL\n10.4 – 11.4"]
        Search["Elasticsearch / OpenSearch\n1 – 8 / 1 – 3"]
        Other["Redis · Valkey · RabbitMQ\nNginx · dnsmasq"]
    end

    User -->|"curl | bash"| InstallSH
    InstallSH --> Installer
    Installer -->|"downloads"| Runtime
    Runtime -->|"installs"| CLIPkg
    Installer -->|"downloads binary"| GoCLI
    CLIPkg -->|"provides"| VenvDir
    GoCLI --> BinDir

    User -->|"valet.sh"| VenvDir
    VenvDir -->|"exec"| GoCLI
    GoCLI -->|"RunSubprocess"| Playbooks
    GoCLI -->|"syscall.Exec (non-TTY)"| Playbooks
    Playbooks -->|"apt / brew / systemd"| Services
    Playbooks -->|"writes"| LogDir

    style ThisRepo fill:#1a1a2e,stroke:#1E90FF,color:#DDDDDD
    style Install fill:#0d0d1a,stroke:#666666,color:#DDDDDD
    style DiskLayout fill:#0d0d1a,stroke:#666666,color:#DDDDDD
    style Services fill:#0d0d1a,stroke:#666666,color:#DDDDDD
```

---

## TUI State Machine

How the interactive launcher transitions between states.

```mermaid
stateDiagram-v2
    [*] --> screenList : valet.sh (no args)
    [*] --> screenList : valet.sh --vi (vim mode on)

    screenList --> screenList : ←/→ navigate\nh/l in vim mode\ntype to filter
    screenList --> screenList : ctrl+[ (toggle vim mode)
    screenList --> screenInline : Enter (any command)
    screenList --> screenList : Enter on subcommand (push nav stack)
    screenList --> [*] : q / Esc at root

    screenInline --> screenInline : type (args into header input)\nctrl+d/u/f/b (scroll docs)
    screenInline --> screenList : Esc (close box)
    screenInline --> screenExec : Enter (execute command)

    screenExec --> screenExec : ↑/↓ scroll log
    screenExec --> screenLogViewer : Y/y (on error, skip prompt)\nY (after prompt shows)
    screenExec --> [*] : any key (after success)\nn/Esc/q/ctrl+c (decline log viewer)

    screenLogViewer --> screenLogViewer : ↑/↓ scroll
    screenLogViewer --> [*] : q / Esc
```

---

## Execution Flow

What happens when a command runs, from user input to Ansible output.

```mermaid
sequenceDiagram
    participant U as User
    participant TUI as TUI (launcher.go)
    participant Inline as InlineBox (inline.go)
    participant Exec as ExecModel (exec.go)
    participant AP as ansible-playbook
    participant Log as debug.log

    U->>TUI: press Enter on "service"
    TUI->>Inline: open InlineBox("service", docs)
    U->>Inline: type "start php83" in header
    U->>Inline: press Enter
    Inline->>Exec: NewExecModel("service start php83")
    Exec->>AP: RunSubprocess() → cmd.Start()
    AP-->>Log: writes task output continuously

    loop every 50ms (execTickMsg)
        Exec->>Log: poll for new lines
        Log-->>Exec: TASK [...] lines + output lines
        Exec->>Exec: if TASK line: tasksDone++, set nextTask
        Exec->>Exec: if 250ms+ since currentTask set: currentTask = nextTask
        Exec->>TUI: render: spinner · current task · log viewport
    end

    AP-->>Exec: process exits (execDoneMsg)
    Exec->>Exec: final log drain

    alt success
        Exec->>U: "✔ N tasks completed  (press any key)"
    else failure
        Exec->>U: "✘ failed — see debug.log\nView full log? [Y/n]"
        U->>Exec: Y
        Exec->>Log: tailFile() last 10,000 lines
        Exec->>U: full-screen log viewer
    end
```

---

## Colour Palette

All colours use terminal palette indices so the TUI adapts to the user's terminal theme.

| Index | Name | Role in TUI |
|---|---|---|
| `12` | Bright blue | Headers (`▶ valet.sh`), filter prompt, section titles |
| `10` | Bright green | Selected command (`▶`), spinner, task counter, `✔ done` |
| `9` | Bright red | `✘ failed`, error messages |
| `8` | Bright black (dim) | Ghost text, separators `·`, dim hints, unselected commands |
| `7` | Normal foreground | Regular list items, input text, log content |

These indices match the ANSI codes used by the Ansible Python callback plugin
(`plugins/callback/valet-sh.py`) so the TUI and Ansible output feel visually consistent.

---

## CLI Task Display: Real-Time Streaming from Ansible stdout

### Problem Solved

The CLI execution panel needed to display **current Ansible task names in real time** as Ansible moves from task to task, without stalling or showing stale information during long-running operations.

### Root Cause Analysis

The initial implementation streamed task names from Ansible's stdout pipe but task names were not appearing in real time. Investigation revealed:

**Python's Buffering Behavior:**
- The Ansible callback plugin (`/usr/local/valet-sh/valet-sh/plugins/callback/valet-sh.py`) writes task names via `print(..., end="\r")`
- When stdout is connected to a pipe (not a TTY), Python defaults to **full buffering** (8 KB buffer)
- Without explicit `sys.stdout.flush()`, task name writes sit in the buffer until:
  1. Buffer fills (rare; happens after ~100 tasks)
  2. Process exits (common case — all buffered output arrives at once, or not at all for interrupted runs)
- Result: task display showed nothing for entire long-running operations, then suddenly all tasks at the very end

### Solution: Force Line Buffering

Set `PYTHONUNBUFFERED=1` in the Ansible subprocess environment. This forces Python to use **line buffering** for stdout when connected to a pipe, ensuring each `print()` call flushes immediately to the pipe.

**Implementation:**
- `cli/internal/ansible/runner.go:248` — added `"PYTHONUNBUFFERED=1"` to the subprocess environment passed to `ansible-playbook`

**Effect:**
- Each task arrival now generates a separate read from the stdout pipe in real time
- Go goroutine `readTaskCmd()` receives data like `"\x1b[2K\r⠙ taskname\r"` (flush + spinner animation + task name)
- Progress bar updates instantly as Ansible transitions between tasks
- Display naturally reflects what Ansible is currently executing

### Data Flow

```
Ansible callback (stdout pipe)
  ↓
PYTHONUNBUFFERED=1 forces flush
  ↓
Go readTaskCmd() reads "\x1b[2K\r⠙ taskname\r"
  ↓
parseAnsibleTaskLine() extracts "taskname"
  ↓
currentTask updated
  ↓
Progress bar re-renders with new task name
```

### Defensive Parsing Improvements

To handle edge cases and ensure robust task name extraction:

1. **`readTaskCmd()` Internal Loop (exec.go:~635)**
   - Changed behavior: only return on IO error/EOF, not on empty parse results
   - Reason: flush-only reads (`"\x1b[2K\r"` with no task name) would trigger spurious EOF detection
   - Now loops internally reading until it gets a real task name or actual EOF

2. **`parseAnsibleTaskLine()` Spinner Validation (exec.go:~656)**
   - Added `spinnerRunes` map with known braille characters: `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`
   - Validates that extracted task names start with a valid spinner rune
   - Prevents play-start lines (`▶ Play Name`) from being misidentified as task names
   - Ensures only actual Ansible task lines are processed

**Test Coverage (`exec_test.go`):**
- 7 test cases for `parseAnsibleTaskLine` covering: normal tasks, flush-only lines, play-start lines, mixed input
- `TestReadTaskCmdSkipsFlushLines` verifies the internal loop behavior
- All tests pass

### Code Changes

**Files modified:**
- `cli/internal/ansible/runner.go` — `PYTHONUNBUFFERED=1` environment variable
- `cli/internal/tui/exec.go` — improved `readTaskCmd()` loop + `parseAnsibleTaskLine()` validation
- `cli/internal/tui/exec_test.go` — expanded test coverage

**Commits:**
- `360c34c` — Initial stdout pipe streaming (had buffering issue)
- `6af1c41` — Previous log-file-based approach (was working but slower)
- Latest — `PYTHONUNBUFFERED=1` fix + defensive parsing improvements (**current design**)

### Why This Works

- **Immediate feedback**: task names appear as soon as Ansible writes them, not minutes later
- **Handles long operations**: when Ansible blocks on a long task (e.g., RabbitMQ startup), the display shows which task is blocking
- **No stale data**: current task always reflects what `ansible-playbook` is executing right now
- **Single-line fix**: `PYTHONUNBUFFERED=1` is minimal, maintainable, and doesn't require modifications to the Ansible callback plugin

### Log Viewer Prompt Interaction

Fixed the "View full log?" prompt to open on single keypress:

1. **Previous design**: 
   - First keypress on error → shows "View full log? [Y/n]" prompt (consumes key)
   - Second keypress (y) → opens log viewer
   - Result: required two keypresses to view log

2. **Current design**:
   - If first keypress is `y/Y` → skip prompt, directly call `loadLogCmd()` (single keystroke)
   - If first keypress is other key → show prompt and wait for response
   - Result: single `y` or `Y` opens log immediately

**Code location:** `cli/internal/tui/exec.go:handleKey()` — error-state branch checks for y/Y and bypasses intermediate prompt state.

**Commit:** `49c1675`
