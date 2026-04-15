# valet-sh

Local development environment manager for Magento, PHP, Neos, AEM, and OroCRM
projects. Manages multiple simultaneous versions of PHP (5.6–8.5),
MariaDB/MySQL, Elasticsearch/OpenSearch, Redis/Valkey, RabbitMQ, and Nginx on
Ubuntu and macOS — both Intel and Apple Silicon.

Provisioning is driven by Ansible playbooks. A Go CLI provides the user-facing
interface, TUI, validation, and update management.

---

## Installation

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/valet-sh/install/master/install.sh)
```

Supported platforms:
- Ubuntu (amd64, arm64)
- macOS Intel (`darwin/amd64`)
- macOS Apple Silicon (`darwin/arm64`) — native, no Rosetta2 required

---

## Repository Ecosystem

valet-sh is spread across four repositories that work together:

```
valet-sh/install      one-liner curl bootstrap script
valet-sh/installer    Go binary: machine setup, downloads the Go CLI
valet-sh/cli          Python package: bash wrapper (delegates to Go CLI)
valet-sh/runtime      Python venv tarball (Ansible + dependencies)
valet-sh/valet-sh     THIS REPO: Ansible playbooks + Go CLI source (cli/)
```

The Go CLI lives in `cli/` inside this repo. The Ansible playbooks live in
`playbooks/` and `roles/`. When a user runs `valet.sh`, the bash wrapper in the
Python venv calls the Go binary, which orchestrates the appropriate Ansible
playbook.

---

## Branch Layout

| Branch | Purpose |
|---|---|
| `2.x` | Production (origin default) |
| `next` | Active development — Go CLI + TUI work |
| `ansible-lint` | Ansible static analysis CI (pending merge) |

---

## Architecture

See [docs/architecture.md](docs/architecture.md) for:
- System overview (4-repo ecosystem + install flow)
- TUI state machine diagram
- Execution sequence diagram
- Colour palette reference

---

## What Has Been Built

### Go CLI (`cli/`)

Built from scratch alongside the existing Ansible playbooks:

- **17 commands** wrapping Ansible playbooks with typed argument validation
- **`.valet-sh.yml` parser** — typed Go structs, validation with clear errors
- **Contextual help** — unknown commands and missing args show relevant help
- **Styled output** — matches the existing Ansible callback colour palette
  (blue `▶` headers, green selection, red `✘` errors)
- **Weekly update check** — polls GitHub Releases API, prompts `[Y/n]`,
  re-execs the original command after updating
- **Bubble Tea TUI** — compact inline launcher (`valet.sh` / `valet.sh --vi`):
  - Single horizontal scrollable command bar (`←/→` or `h/l` in vim mode)
  - Header shows ghost text of the currently hovered command
  - `ctrl+[` toggles vim mode (`hjkl` navigation); `[VIM]` indicator left of version
  - `valet.sh --vi` launches directly in vim mode
  - Inline box opens below selected command: ghost prompt (`valet.sh <cmd> █`) +
    scrollable docs (`ctrl+d/u/f/b`) — always insert mode, no modal switching
  - Fuzzy filter: type any character to search commands
  - Live execution panel (full-width, inline — no alt-screen)
  - Log viewer on failure: `View full log? [Y/n]` → scrollable viewport
  - Terminal palette colours (adapts to user's theme)
  - TTY-aware: panel on TTY, `syscall.Exec` fallback for CI/pipes
- **Shell completions** (planned) — bash/zsh/fish via cobra
- **60+ unit tests** across 6 packages
- **CI/CD**: golangci-lint v1.64.8, gofmt, go vet, race detector, Codecov,
  cross-platform release pipeline (4 binaries on `v*` tags)

### Wiring (committed locally, not yet released)

Changes to `valet-sh/cli`, `valet-sh/runtime`, and `valet-sh/installer` are
committed locally and ready to push. The first release tag has not been cut.

---

## Open Todos

Security audits are deferred — flag dangerous patterns during development,
comment suspicious code, consult the user before proceeding.

| Item | Priority |
|---|---|
| Checksum verification in installer download | **High** (done ✓) |
| Security audit: RCE surface through subprocess args | **High** (deferred) |
| Supply chain: pin GitHub Actions to commit SHAs | **High** (deferred) |
| Supply chain: audit charm ecosystem deps | **High** (deferred) |
| Architecture diagram | Medium (done ✓) |
| Implement progress bar in execution panel | Medium (done ✓) |
| Shell completions — bash/zsh/fish via cobra, install on setup/update | Medium |
| Add security-focused tests | Medium |
| TUI: `<Tab>` to toggle between horizontal scroll and vertical list view | Low |
| TUI grid layout view (2-column alternative to horizontal scroll) | Low |
| Convert Ansible roles to native Go commands where it makes sense | Long-term |
| Cut first release tag | Low |
| Push `valet-sh/cli`, `runtime`, `installer` changes | Low |
| Merge `ansible-lint` branch | Low |
| Fix Password on TUI | High |
| Password doesn't get asked when running a bare command | High |
| When running bare commands the tui takes over completely | High |

---

## Code Style

These rules apply to all Go code in `cli/`. They were established through
development and exist to keep the codebase readable without IDE support.
Future contributors — human or AI — must follow them. If a new pattern
emerges that isn't covered here, update this section.

### Naming: field names must be self-documenting

The type annotation on a field tells you *what* it is. The name must tell you
*which one and why* — without needing to look at the type or surrounding
context.

```go
// BAD — "vp" tells you nothing beyond the type
vp viewport.Model

// GOOD — "viewport" is the live log panel during execution;
// its counterpart "logViewer" is the full-screen viewer on failure
viewport  viewport.Model
logViewer viewport.Model
```

```go
// BAD — "current" what?
current list.Model

// GOOD — clearly the list of available commands
commandList list.Model
```

```go
// BAD — meaningless abbreviation
var sb strings.Builder

// GOOD — communicates that it's accumulating rendered output
var output strings.Builder
```

**One-letter receivers are fine.** `e` for `ExecModel`, `m` for `model`,
`p` for `ArgPane` — these are standard idiomatic Go. The rule applies to
struct fields and local variables, not receiver names.

### Naming: local variable extraction

Only extract a local variable when the value is used in two or more places in
the same function. If it is used once, inline the call.

```go
// BAD — extracted but only used once; the method name already says everything
leftWidth := m.listWidth()
left := styles.LeftPane.Width(leftWidth).Render(content)

// GOOD — inline it
left := styles.LeftPane.Width(m.listWidth()).Render(content)

// GOOD — extracted because it's used twice
leftWidth := m.listWidth()
left := styles.LeftPane.Width(leftWidth).Render(content)
m.stack[i].list.SetSize(leftWidth, listHeight)
```

### Constants: no magic numbers

Every numeric or string constant must be named and accompanied by a comment
explaining its purpose and unit. This makes layout arithmetic readable.

```go
// GOOD
const (
    // execHeaderHeight: command line + divider.
    execHeaderHeight = 2

    // execFooterHeight: divider + status/hint line.
    execFooterHeight = 2

    // execFooterHeightPrompt: divider + failure line + prompt line.
    // Used when awaiting the "View full log?" response so the viewport
    // does not jump when the prompt appears.
    execFooterHeightPrompt = 3

    // logViewMaxLines is the maximum number of lines loaded from the log
    // file when the viewer is opened after a failure.
    logViewMaxLines = 10_000
)

// BAD — reader has no idea what 4 and 2 mean
h := totalHeight - 4 - 2
```

### Linting: never use `//nolint` in source

`//nolint` comments scatter suppression decisions across the codebase where
future readers cannot see why they exist. Instead, add a targeted exclusion to
`.golangci.yml` with a comment explaining the reasoning.

```go
// BAD
_ = cmd.Help() //nolint:errcheck
```

```yaml
# GOOD — in .golangci.yml, with explanation
- path: internal/commands/helpers.go
  linters:
    - errcheck
  # cmd.Help() writes to stdout which is best-effort; errors are non-critical
```

### Bubble Tea: value receivers are required

The Elm architecture mandates that `Update()` returns a **new model copy**
rather than mutating the receiver. All Bubble Tea model methods must use
value receivers. The `gocritic` `hugeParam` warning is suppressed for
`internal/tui/` in `.golangci.yml` because this is intentional by design.

```go
// CORRECT — value receiver, returns updated copy
func (e ExecModel) Update(msg tea.Msg) (ExecModel, tea.Cmd) { ... }

// WRONG — do not convert to pointer receiver to silence the warning
func (e *ExecModel) Update(msg tea.Msg) (ExecModel, tea.Cmd) { ... }
```

### Error handling

| Situation | Pattern |
|---|---|
| Critical path | `return err` or `return fmt.Errorf("context: %w", err)` |
| Best-effort output (`fmt.Fprintln`) | `_, _ = fmt.Fprintln(w, ...)` |
| Best-effort file close in defer | `_ = f.Close()` with comment if non-obvious |
| User-facing CLI errors | `commands.ErrorPrefix("message")` for red `✘` |

### Testing

- Use table-driven tests for all pure logic
- Use `t.TempDir()` for any test that creates files
- File permissions: `0o644` not `0644` (Go 1.13+ octal literal syntax)
- Race detector is always on in CI: `go test -race ./...`

### Spellings

Use US English throughout:
- `behavior` not `behaviour`
- `color` not `colour`
- `synchronize` not `synchronise`
- `initialize` not `initialise`

---

## For AI Agents

Read `cli/.opencode/AGENTS.md` before making any changes to the Go CLI.
It contains critical rules, code patterns, and the verification checklist
that must be run before any commit.

Read `cli/.opencode/SKILLS.md` for domain knowledge about the valet-sh
architecture, the TUI package structure, and the relationship between repos.

**If you add a feature that changes architecture, naming conventions, or
introduces new patterns not covered by this document — update this README,
AGENTS.md, and SKILLS.md in the same commit. If unsure whether the change
warrants a docs update, ask the user.**

---

## License

Apache 2.0 — see [LICENSE](LICENSE) for full text.
Copyright 2025 TechDivision GmbH
