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

## CLI Task Display (Hold-Timer Model)

### Problem Solved

The CLI execution panel needed to display the current Ansible task in a human-readable way:

1. **Initial Design (Queue Model)**: Buffered all discovered tasks in a queue, draining one per 50ms tick (every tick). This caused:
   - Queue exhaustion: tasks from the first batch (300ms of log output) were consumed in 2-3 seconds
   - Freeze during long tasks: once the queue emptied, display froze at "ensure rabbitmq is started" (or similar) until the next new task appeared, which could be 30-60+ seconds later
   - Historical replay: showed history of completed tasks rather than what Ansible is currently doing

2. **Second Design (Paced Queue)**: Added a 250ms hold timer per task (dequeue one every 5 ticks). This helped but still suffered from:
   - Queue drain dependency: tasks still queued up and drained, causing same freeze issue
   - Disconnected from reality: showing historical tasks while long operations run

### Solution: Hold-Timer Model

Redesigned to show **what Ansible is currently doing**, not a queue of historical tasks.

**Fields in `ExecModel`:**
- `currentTask string` — task currently being displayed
- `nextTask string` — most recently discovered task waiting to show
- `lastTaskChangedAt time.Time` — timestamp of last display update

**Logic:**

1. When a new `TASK [...]` line appears in the log:
   - Set `nextTask = parsed_task_name`
   - If `currentTask` is empty (first task), immediately set `currentTask = nextTask`

2. On each 50ms tick (execTickMsg):
   - If `nextTask != ""` AND `time.Since(lastTaskChangedAt) >= 250ms`:
     - Advance: `currentTask = nextTask`, clear `nextTask`, update timestamp

3. On process completion (execDoneMsg):
   - Flush any pending `nextTask` to `currentTask` so final task is always shown

**Result:**
- **No queue exhaustion**: only one "pending" task is kept (`nextTask`)
- **Responsive to Ansible**: displays what's currently running, not historical replay
- **Human-readable pacing**: each task shown for minimum 250ms, fast tasks each get visibility, slow tasks stay showing naturally
- **Always accurate**: if a long task takes 60 seconds, it displays for 60 seconds (no disconnect)

**Commits:**
- `517fd52`: Initial task queue implementation (superseded)
- `3e42896`: Queue pacing refinement (superseded)
- `49c1675`: Replace with hold-timer model ← **current design**

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

**Commit:** `49c1675` (same commit as hold-timer redesign)
