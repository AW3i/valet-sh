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
    screenExec --> screenLogViewer : Y (after failure prompt)
    screenExec --> [*] : any key (after success)\nn/Esc (decline log viewer)

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

    loop every 100ms
        Exec->>Log: poll for new lines
        Log-->>Exec: TASK [...] lines
        Exec->>Exec: tasksDone++ on each TASK line
        Exec->>TUI: render: spinner · N tasks · log viewport
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
