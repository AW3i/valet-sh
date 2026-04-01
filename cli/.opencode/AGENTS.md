# AGENTS.md — Instructions for AI Assistants

This file is your operational contract for working on the valet-sh Go CLI.
Read it before touching any code. Update it when you establish new patterns.

---

## Critical Rules

### 1. NO `//nolint` comments

Never add `//nolint` or `// nolint` in source files. Instead, fix the issue
properly or add a targeted exclusion to `.golangci.yml` with an explanation.

```go
// BAD
_ = cmd.Help() //nolint:errcheck

// GOOD — just write the code
_ = cmd.Help()
// then in .golangci.yml:
// - path: internal/commands/helpers.go
//   linters: [errcheck]
//   # cmd.Help() writes to stdout which is best-effort
```

### 2. Always run full verification before committing

```bash
cd cli
make lint           # golangci-lint
go test -race ./... # tests with race detector
go build ./...      # compile check
```

All three must pass. If any fail, fix before committing.

### 3. Conventional commit messages

- Format: `type: short description` (under 72 chars)
- Types: `feat`, `fix`, `test`, `ci`, `refactor`, `docs`
- Body optional but useful for non-obvious changes

```
feat: add checksum verification to installer download
fix: resolve ansible-playbook path from venv not runtime
refactor: expand abbreviated names across TUI package
```

### 4. Squash iterative fix commits before pushing

Multiple fix commits made during development should be squashed into one
logical commit before the user pushes to the remote.

```bash
git reset --soft <first-commit-of-the-work>
git add -A
git commit -m "feat: descriptive message of the whole change"
```

### 5. Always update docs when context changes

If you add a package, introduce a new pattern, change architecture, or rename
something significant:

1. Update the root `README.md` if it affects the whole project
2. Update `cli/README.md` if it affects the CLI specifically
3. Update this `AGENTS.md` if it changes how agents should work
4. Update `SKILLS.md` if it changes domain knowledge or architecture

Do this **in the same commit** as the code change. If you are unsure whether
a change warrants a documentation update, **ask the user**.

---

## Naming Rules

These rules exist so code is readable without IDE support or type context.

### Field names must be self-documenting

The type annotation tells you *what* a field is. The name must tell you
*which one and why* — without needing to look at the surrounding struct.

```go
// BAD — "vp" adds nothing beyond the type
vp viewport.Model

// GOOD — role is clear; its counterpart logViewer is the post-failure viewer
viewport  viewport.Model
logViewer viewport.Model
```

```go
// BAD — current what?
current list.Model

// GOOD
commandList list.Model
```

```go
// BAD
var sb strings.Builder

// GOOD — communicates it's accumulating rendered output for return
var output strings.Builder
```

### One-letter receivers are fine

`e` for `ExecModel`, `m` for `model`, `p` for `ArgPane` — idiomatic Go.
The field naming rule applies to struct fields and local variables, not receivers.

### Extract local variables only when used 2+ times

If a value is used once, inline it. The method name already documents intent.

```go
// BAD — extracted but used only once
leftWidth := m.listWidth()
left := styles.LeftPane.Width(leftWidth).Render(content)

// GOOD — inline
left := styles.LeftPane.Width(m.listWidth()).Render(content)

// GOOD — extracted because used twice
leftWidth := m.listWidth()
left := styles.LeftPane.Width(leftWidth).Render(content)
m.stack[i].list.SetSize(leftWidth, listHeight) // second use
```

### Common abbreviations to avoid

| Avoid | Use instead | Why |
|---|---|---|
| `vp` | descriptive name (`viewport`, `logViewer`) | type already says Model |
| `sb` | `output` | Builder for what? |
| `lw`, `rw` | `leftWidth`, inline `m.descWidth()` | one or two letters, no meaning |
| `ver` | `versionLabel` | ver is ambiguous |
| `pad` | `versionPadding`, `headerPadding` | padding for what? |

---

## Constants: no magic numbers

Every numeric constant must be named and documented with a comment explaining
its purpose and unit. This makes layout arithmetic readable without a calculator.

```go
const (
    // execHeaderHeight: command line + divider.
    execHeaderHeight = 2

    // execFooterHeightPrompt: divider + failure line + prompt line.
    // Larger than execFooterHeight so the viewport does not jump
    // when the "View full log?" prompt appears.
    execFooterHeightPrompt = 3

    // logViewMaxLines is the maximum lines loaded from the log file
    // when the viewer is opened after failure. Keeps rendering fast
    // for large log files accumulated over many runs.
    logViewMaxLines = 10_000
)
```

---

## Bubble Tea Pattern

The Elm architecture requires `Update()` to return a **new model copy**.
All Bubble Tea model methods must use **value receivers**. Never convert to
pointer receivers to silence a warning.

```go
// CORRECT
func (e ExecModel) Update(msg tea.Msg) (ExecModel, tea.Cmd) { ... }

// WRONG — breaks immutability contract
func (e *ExecModel) Update(msg tea.Msg) (ExecModel, tea.Cmd) { ... }
```

The `gocritic` `hugeParam` warning is suppressed for all files in
`internal/tui/` via `.golangci.yml`. This is expected and intentional.

When adding a new Bubble Tea model:
1. Use value receivers on all methods
2. Add a `hugeParam` exclusion for the new file in `.golangci.yml`
3. Add a comment in the struct explaining this is intentional

---

## Code Patterns

### Adding a new command

1. Create `internal/commands/<name>.go`
2. Function: `func NewXxxCmd() *cobra.Command`
3. Use `&ansible.RunOpts{}` (pointer, not value — avoids `hugeParam`)
4. Register in `cmd/valet/main.go` `cmd.AddCommand()` list

```go
func NewXxxCmd() *cobra.Command {
    var verbose bool
    cmd := &cobra.Command{
        Use:   "xxx <required> [optional]",
        Short: "One sentence description",
        Args:  requireArgs(1, 2), // shows help on too few, error on too many
        RunE: func(cmd *cobra.Command, args []string) error {
            return ansible.Run(&ansible.RunOpts{
                Playbook: "xxx",
                Args:     args,
                Verbose:  verbose,
            })
        },
    }
    cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
    return cmd
}
```

### Error handling

| Situation | Pattern |
|---|---|
| Critical path | `return fmt.Errorf("context: %w", err)` |
| Best-effort stdout (`fmt.Fprintln`) | `_, _ = fmt.Fprintln(w, ...)` |
| Best-effort file close in defer | `_ = f.Close()` |
| User-facing CLI errors | `commands.ErrorPrefix("message")` → red `✘` |

### Output rendering (help, TUI)

Use wrapper functions that explicitly discard errors:

```go
// in help.go
func printLine(w io.Writer, a ...any) {
    _, _ = fmt.Fprintln(w, a...)
}
```

---

## Testing

- Table-driven tests for all pure logic
- `t.TempDir()` for any test that touches the filesystem
- File permissions: `0o644` not `0644` (Go 1.13+ octal literal syntax)
- Race detector always on: `go test -race ./...`
- Test files alongside source: `foo.go` → `foo_test.go`, same package

```go
func TestParseSemver(t *testing.T) {
    tests := []struct {
        input    string
        expected [3]int
    }{
        {"2.9.19", [3]int{2, 9, 19}},
        {"2.9.19-101-gabcdef", [3]int{2, 9, 19}},
        {"3.0.0", [3]int{3, 0, 0}},
    }
    for _, tc := range tests {
        t.Run(tc.input, func(t *testing.T) {
            result := parseSemver(tc.input)
            if result != tc.expected {
                t.Errorf("parseSemver(%q) = %v, want %v", tc.input, result, tc.expected)
            }
        })
    }
}
```

---

## Linter Configuration

### Current exclusions (intentional)

| Path | Linter | Reason |
|---|---|---|
| `internal/commands/help.go` | `errcheck` | `fmt.Fprintln` to stdout is best-effort |
| `internal/commands/helpers.go` | `errcheck` | `cmd.Help()` stdout errors non-critical |
| `internal/updater/check.go` | `errcheck` | File close, HTTP body close, `syscall.Exec` — all best-effort |
| `internal/tui/` | `gocritic/hugeParam` | Bubble Tea value receivers — required by design |
| `internal/tui/exec.go` | `errcheck` | Best-effort file/viewport ops |
| `internal/tui/exec.go` | `gosec/G304` | `tailFile()` receives hardcoded `logPath` constant only |
| `internal/tui/runner.go` | `errcheck` | Best-effort workdir lookup |
| Global | `gosec/G204` | `syscall.Exec` with trusted argv is intentional |

### Adding a new exclusion

1. Add it to `.golangci.yml` under `issues.exclude-rules`
2. Add a comment explaining **why** the suppression is intentional
3. Keep it as targeted as possible (specific file, not package-wide if avoidable)
4. Document it in the table above

---

## File Permissions

Use Go 1.13+ octal syntax:

```go
os.WriteFile(path, data, 0o644)  // GOOD
os.MkdirAll(path, 0o755)         // GOOD
os.WriteFile(path, data, 0644)   // BAD — old style
```

---

## Spellings

US English throughout:

| Use | Not |
|---|---|
| `behavior` | `behaviour` |
| `color` | `colour` |
| `synchronize` | `synchronise` |
| `initialize` | `initialise` |

---

## Common Pitfalls

**Variable shadowing** — never use `min`, `max`, `len`, `cap` as variable
names. They shadow builtins.

```go
// BAD
func requireArgs(min, max int) { ... }

// GOOD
func requireArgs(minArgs, maxArgs int) { ... }
```

**Defer in loops** — `defer` in a loop defers until the *function* returns,
not the iteration. Either extract to a helper function or call explicitly.

**Unused imports** — remove any import whose last use is deleted during
refactoring. `go build` will catch it but clean it up before committing.

**`strings.Builder` reset** — if you reuse a builder in a loop, call
`builder.Reset()` between iterations.

---

## Security

The CLI uses `syscall.Exec()` to replace itself with `ansible-playbook`:
- Intentional for signal handling (Ctrl-C reaches Ansible directly)
- Safe: `argv` is built from platform constants + cobra-parsed args, not raw user input
- `tailFile()` receives only the hardcoded `logPath` constant

When modifying subprocess code:
- Never pass user-controlled strings directly into `argv` without cobra parsing them first
- Never construct file paths from user input without validation
- Review any new `os.Open` or `exec.Command` calls for G304/G204 implications
- Document the rationale if a new gosec exclusion is needed

---

## When to Ask the User

Ask the user before proceeding when:

1. A naming decision is genuinely ambiguous (e.g., two plausible names with different trade-offs)
2. A change touches more than one repository in the ecosystem
3. A new architectural pattern is introduced that isn't covered by existing docs
4. A security-relevant change is needed (subprocess args, file paths, downloads)
5. A large refactor would affect many files and you're unsure of intended scope
6. This AGENTS.md doesn't cover the situation you're facing

It is always better to ask one targeted question than to make a wrong assumption
that requires undoing work.
