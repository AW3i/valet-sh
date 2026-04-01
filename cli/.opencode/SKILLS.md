# SKILLS.md — Domain Knowledge for valet-sh CLI

This file contains architectural knowledge, domain context, and design
rationale. Read it when you need to understand *why* things are the way they
are, not just how to work with them.

---

## What valet-sh Does

valet-sh provisions and manages local development environments for Magento,
PHP, Neos, AEM, and OroCRM projects. It manages multiple simultaneous versions
of:

- PHP 5.6 – 8.5
- MariaDB 10.4 – 11.4 / MySQL 5.7 – 8.4
- Elasticsearch 1 – 8 / OpenSearch 1 – 3
- Redis / Valkey
- RabbitMQ
- Nginx + dnsmasq

It runs on Ubuntu (amd64, arm64) and macOS (Intel `darwin/amd64`, Apple
Silicon `darwin/arm64`).

---

## Architecture Overview

```
User
  │
  │  runs valet.sh
  ▼
valet.sh (bash)                 /usr/local/valet-sh/venv/bin/valet.sh
  │                             Thin wrapper: exec's Go CLI binary
  ▼
valet (Go CLI binary)           /usr/local/valet-sh/bin/valet
  │
  ├── No args?    → TUI launcher (Bubble Tea, interactive)
  │
  ├── Args, TTY?  → Execution panel (Bubble Tea, shows debug.log)
  │                 starts ansible-playbook as subprocess
  │
  └── Args, pipe? → syscall.Exec into ansible-playbook directly
                    (Go process replaced, signals flow through)
  ▼
ansible-playbook                /usr/local/valet-sh/venv/bin/ansible-playbook
  │
  └── Playbooks + roles         /usr/local/valet-sh/valet-sh/
        └── Manages services on the host OS (macOS/Ubuntu)
```

### Why Go + Ansible?

The tool was originally pure Ansible/bash. The Go CLI was added to provide:

1. Typed `.valet-sh.yml` validation with clear error messages
2. Better UX: contextual help, TUI, styled output
3. Update checking and self-management
4. A foundation for gradually replacing Ansible roles with native Go where it
   makes sense (long-term goal)

Ansible still handles all provisioning, service management, and bootstrapping.
Go orchestrates it.

---

## Repository Ecosystem

| Repo | Purpose |
|---|---|
| `valet-sh/install` | One-liner `curl \| bash` bootstrap |
| `valet-sh/installer` | Go binary: machine setup, downloads Go CLI binary |
| `valet-sh/cli` | Python package: bash wrapper (`valet.sh`), delegates to Go CLI |
| `valet-sh/runtime` | Python venv tarball: Ansible + pip dependencies |
| `valet-sh/valet-sh` | **This repo**: Ansible playbooks + Go CLI source |

### Release sequence

```
1. Tag valet-sh/valet-sh  →  GitHub Actions builds 4 binaries
2. Tag valet-sh/cli       →  bash wrapper updated (exec to Go binary)
3. Tag valet-sh/runtime   →  venv tarball rebuilt with new cli package
4. Tag valet-sh/installer →  installer downloads new Go binary on setup/update
```

---

## Installation Layout

```
/usr/local/valet-sh/
├── bin/
│   └── valet                    Go CLI binary (from this repo)
├── etc/
│   ├── config.yml               Global config (hub_domain, development_tld)
│   ├── links.yml                Active vhost symlinks
│   └── .last_update_check       Mtime = timestamp of last update check
├── installer/
│   └── valet-sh-installer       Bootstrap installer (valet-sh/installer)
├── packages/                    Homebrew packages, downloaded tarballs
├── valet-sh/
│   ├── playbooks/               Ansible playbooks (from this repo)
│   ├── roles/                   Ansible roles (from this repo)
│   └── log/
│       └── debug.log            Ansible callback output (tailed by exec panel)
└── venv/
    └── bin/
        ├── ansible-playbook     Ansible from Python venv
        └── valet.sh             Bash wrapper → execs Go binary
```

---

## .valet-sh.yml Format

Project configuration file. Format is stable — never change keys without
backwards compatibility.

```yaml
hub:
  host: "git.example.com"
  port: 22
  path: "/data"

services:
  php:
    version: 8.1
  mariadb:
    version: 10.6
    database: magento_prod_copy
  elasticsearch:
    version: 7
    plugins: ["analysis-icu"]
  redis: {}

instance:
  key: "myproject"        # Becomes hostname: myproject.test
  type: "magento2"        # Bootstrap workflow type
  path: "src"             # Docroot relative to project root

  multidomain:
    "de.magento.test": "de_DE"

  sync:
    identifier: "staging"
    db: true
    fs: ["pub/media", "var/log"]
```

### Instance types

| Type | Bootstrap behaviour |
|---|---|
| `magento2` | `env.php` generation, indexer setup, cache clear |
| `magento1` | Magento 1 local XML config |
| `neos` | Neos CMS setup |
| `aem` | Adobe Experience Manager |
| `orocrm` | OroCRM |

### Service fuzzy aliases

The CLI normalises service names before passing to Ansible:
- `PHP8.3`, `php8.3`, `PHP83` → `php83`
- `mariadb10.4`, `MARIADB10.4` → `mariadb104`
- `mysql5.7` → `mysql57`

See `platform.NormalizeServiceName()` for the full mapping.

---

## TUI Package (`internal/tui/`)

### Files and responsibilities

| File | Responsibility |
|---|---|
| `launcher.go` | Root Bubble Tea model. Navigation stack (`stackEntry`), screen state machine, `commandList` management. Hosts `ExecModel` when executing. |
| `list.go` | `CommandItem` implements `bubbles/list.Item`. Custom delegate with valet-sh colours. `argsFromUse()` parses cobra `Use` strings for arg pane. |
| `args.go` | `ArgPane` — one `bubbles/textinput` per argument. Tab navigation between fields. `IsReady()` checks required fields. |
| `exec.go` | `ExecModel` — live log panel + log viewer. See below. |
| `runner.go` | `RunWithPanel()` entry point for direct CLI invocations. `standaloneExecModel` wraps `ExecModel` as a full-screen program. `resolveRunOpts()` maps cobra args to `ansible.RunOpts`. |
| `styles.go` | Lip Gloss styles. Colour values match the Python callback plugin's ANSI codes exactly. |

### Screen state machine

The root `model` in `launcher.go` tracks state with the `screen` type:

```
screenList   Default: user navigates command hierarchy
     │
     │ Enter (leaf, has args)
     ▼
screenArgs   Inline argument input; Tab between fields
     │
     │ Enter (all required filled)  OR
     └───────────────────────────────────┐
                                         │
     (Enter on leaf with no args)        │
     ────────────────────────────────────┘
                                         ▼
screenExec   ExecModel active on right pane (or full screen for direct CLI)
```

Inside `ExecModel`, a separate boolean flag `logViewOpen` controls whether
the full-screen log viewer is shown (distinct from the screen state machine
because it lives entirely within the exec model).

### ExecModel internals

```
ExecModel
  ├── viewport viewport.Model    Rolling live-log panel (100ms poll of debug.log)
  ├── logViewer viewport.Model   Full-screen viewer shown after Y/n prompt
  ├── awaitingLogPrompt bool     True after failure, waiting for Y/n
  └── logViewOpen bool           True once user pressed Y and log is loaded
```

**Log tailing**: on subprocess start, `debug.log` is opened and seeked to
current EOF. A 100ms `tea.Tick` polls for new bytes using `bufio.Scanner`.
Lines are appended to `viewport` content and `GotoBottom()` is called.

**`tailFile()`**: reads up to `logViewMaxLines` (10,000) lines using a ring
buffer — single O(n) pass, O(maxLines) memory, no backward seeking.

**Two viewport fields, not one**: `viewport` is the rolling live output during
execution. `logViewer` is the full-screen post-failure viewer. They are
separate to avoid tearing down and rebuilding a scrolled viewport when the
user switches between them.

### Two Ansible execution paths

| Function | Where used | Mechanism | Why |
|---|---|---|---|
| `ansible.Run()` | Non-TTY / `--help` / unknown command fallback | `syscall.Exec` replaces process | Signals (Ctrl-C) flow to Ansible; Go process disappears |
| `ansible.RunSubprocess()` | TUI execution panel | `exec.Cmd.Start()` returns `*exec.Cmd` | Go stays alive to tail log and render panel |

### Colour palette

All colours in `styles.go` match the Python callback plugin (`plugins/callback/valet-sh.py`):

| Name | Hex | Matches Ansible ANSI |
|---|---|---|
| `colourBlue` | `#1E90FF` | `\033[1;34m` — play start headers |
| `colourGreen` | `#00CC00` | `\033[0;32m` — spinner/ok |
| `colourRed` | `#FF3333` | `\033[1;31m` — failure |

---

## Update Flow

1. `updater.Check()` runs on every command invocation (skipped for `--help`/`--version`)
2. Reads mtime of `/usr/local/valet-sh/etc/.last_update_check`
3. If older than 7 days: fetches GitHub Releases API (`api.github.com/repos/valet-sh/valet-sh/releases/latest`), 3s timeout
4. Writes timestamp regardless (avoids hammering API on network errors)
5. If newer version: prompts `Update now? [Y/n]`
6. On Y: runs `valet-sh-installer update`, then `syscall.Exec` re-runs original command
7. Version comparison uses semver parsing (`major.minor.patch`), ignoring git-describe suffixes

---

## Security Model

### Subprocess args

`ansible.Run()` and `ansible.RunSubprocess()` build `argv` from:
- Absolute path to `ansible-playbook` from `platform.AnsiblePlaybookBin()`
- Absolute path to playbook file from `platform.RepoDir()` + playbook name
- Extra-vars JSON with `CLIVars` struct (serialised from typed fields)
- User CLI args are passed through cobra's parsing layer before reaching argv

No raw user strings reach `exec.Command` or `syscall.Exec` directly.

### File paths

`tailFile()` always receives the `logPath` constant
(`/usr/local/valet-sh/valet-sh/log/debug.log`). The gosec G304 warning is
excluded in `.golangci.yml` with this explanation. If `tailFile` is ever
changed to accept user-provided paths, remove that exclusion and add proper
validation.

### Go module integrity

`go.sum` pins exact SHA-256 hashes (`h1:...`) for every module dependency.
Running `go mod download` verifies these hashes. Dependencies cannot be
silently swapped.

### GitHub Actions (known gap)

CI workflows currently reference actions by mutable tags (`@v4`, `@v5`).
Tags can be moved. To harden against supply chain attacks, pin to full commit
SHAs:

```yaml
# Current (mutable)
uses: actions/checkout@v4

# Hardened (immutable)
uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683  # v4.2.2
```

This is tracked as an open todo.

---

## Version Handling

| Context | Format | Example |
|---|---|---|
| Git tags | `v` prefix | `v2.10.0` |
| Binary (ldflags) | No `v` prefix | `2.10.0` |
| Dev build (git describe) | No `v`, has suffix | `2.9.19-102-g35e11d2` |

`parseSemver()` in `updater/check.go` strips the git-describe suffix
(`-102-g35e11d2`) before comparing versions. `dev` builds (local, no tag)
skip the update check entirely.

---

## Platform Detection

`platform.Detect()` returns an `Info` struct with `OS` (`ubuntu`/`mac`) and
`Arch` (`amd64`/`arm64`). These are passed to Ansible via the `cli` extra-var.
Platform-specific logic lives entirely in Ansible roles, not in Go.

The `valet.sh` installer handles `linux-gnu` as Ubuntu (matching existing
Ansible role behaviour). Linux Mint remapping is handled inside the
`shared-variables` Ansible role.

---

## Common Operations

### Adding a new command

See `AGENTS.md` for the code pattern. Additionally:

1. Create `playbooks/<name>.yml` in the Ansible playbooks directory
2. Create `internal/commands/<name>.go` with `NewXxxCmd()`
3. Register in `cmd/valet/main.go`
4. Add unit tests in `internal/commands/<name>_test.go` if there's logic

### Adding config validation

In `internal/config/project.go`, add to the `Validate()` method:

```go
if c.Services.Redis != nil && c.Services.Valkey != nil {
    errs = append(errs, "services.redis and services.valkey cannot both be set")
}
```

### Adding a new TUI screen

1. Add a constant to the `screen` type in `launcher.go`
2. Handle it in `handleKey()` and `routeMsg()`
3. Add rendering in `render()` or as a separate `xxxView()` method
4. Update `resizeAll()` if the new screen has resizable components
5. Add `hugeParam` exclusion in `.golangci.yml` if a new model struct is added
6. Write tests in `launcher_test.go` or a new `xxx_test.go`

---

## Open Todos

| Item | Notes |
|---|---|
| Cut first release tag | Go binaries not yet downloadable |
| Push cli/runtime/installer changes | Committed locally in /tmp |
| Checksum verification in installer | `checksums.txt` published but not verified on download |
| Pin GitHub Actions to SHAs | Currently using mutable `@v4` tags |
| Audit charm ecosystem deps | bubbletea, bubbles, lipgloss — review changelogs and signatures |
| Search bar in TUI launcher | Filter commands as you type |
| Architecture diagram | System overview + TUI state flow diagram |
| Progress bar in exec panel | Placeholder in `execView()` already exists |
| Security tests | Input sanitisation, path traversal, subprocess arg injection |
| Convert Ansible to native Go | Gradual, where it makes sense |
| Merge `ansible-lint` branch | ansible-lint + syntax check CI |

---

## Domain Terminology

| Term | Meaning |
|---|---|
| `valet-sh` | The project name |
| `valet.sh` | The user-facing command (bash wrapper → Go binary) |
| `init-instance` | Bootstrap a project from `.valet-sh.yml` |
| `link` | Create nginx vhost + SSL cert for current directory |
| `restore` | Sync DB/files from remote hub environment |
| `service` | Manage background services (start/stop/restart/enable/disable) |
| hub | Remote environment (usually staging) used as data source for `restore` |
| instance key | Project hostname prefix (`key: "myproject"` → `myproject.test`) |
