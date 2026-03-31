# AGENTS.md - Instructions for AI Assistants

## Critical Rules

### 1. NO `//nolint` COMMENTS
**NEVER** add `//nolint` or `// nolint` comments to code. Instead:
- Fix the underlying issue properly
- OR add targeted exclusions to `.golangci.yml` if the warning is intentional

Example of what NOT to do:
```go
// BAD - never do this
_ = cmd.Help() //nolint:errcheck
```

Example of correct approach:
```go
// GOOD - just write the code
_ = cmd.Help()
```
Then add to `.golangci.yml`:
```yaml
- path: internal/commands/helpers.go
  linters:
    - errcheck
```

### 2. Always Run Full Verification
Before saying you're done, run ALL of:
```bash
cd cli
make lint        # Run golangci-lint
go test -race ./...  # Run tests with race detector
go build ./...    # Verify everything compiles
```

### 3. Commit Message Style
- Use conventional commits: `type: description`
- Types: `feat`, `fix`, `test`, `ci`, `refactor`, `docs`
- Keep first line under 72 characters
- Reference context from user requests

Good examples:
- `feat: add checksum verification to binary downloads`
- `fix: handle missing .valet-sh.yml gracefully`

### 4. Squash Fix Commits
If you create multiple fix commits during development, squash them into logical commits before the user pushes. Example:
```bash
# If you have:
# a25808e test: comprehensive unit tests
# 10b15f4 fix: resolve all golangci-lint errors
# fead2e4 ci: configure golangci-lint
# 10b15f4 fix: more fixes
# fead2e4 fix: another fix

# Squash to:
git reset --soft a25808e
git commit -m "test: comprehensive unit tests and CI/CD quality gates"
```

## Code Patterns

### Adding New Commands
1. Create function in `internal/commands/<name>.go`
2. Function signature: `func NewXxxCmd() *cobra.Command`
3. Use `&ansible.RunOpts{}` (pointer, not value)
4. Add to `cmd/valet/main.go` in `cmd.AddCommand()` list
5. Pattern:
```go
func NewXxxCmd() *cobra.Command {
    var verbose bool
    cmd := &cobra.Command{
        Use:   "xxx",
        Short: "Brief description",
        RunE: func(cmd *cobra.Command, args []string) error {
            // ... validation ...
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

### Error Handling
- Critical errors: return them
- Best-effort operations (help, file close, HTTP): use blank assignment `_ = ...`
- User-facing errors: use `commands.ErrorPrefix()` for red ✘ styling

### Output Helpers
For help output (best-effort, ignore errors):
```go
_, _ = fmt.Fprintln(w, text)
_, _ = fmt.Fprintf(w, "format", args...)
```

## Testing

### Adding Tests
- Create `xxx_test.go` alongside source
- Use table-driven tests
- Test file paths use `t.TempDir()` for isolation
- File permissions: use `0o644` not `0644` (octal literals)

Example:
```go
func TestParseSemver(t *testing.T) {
    tests := []struct {
        input    string
        expected [3]int
    }{
        {"2.9.19", [3]int{2, 9, 19}},
        {"2.9.19-101-gabcdef", [3]int{2, 9, 19}},
    }
    for _, tc := range tests {
        t.Run(tc.input, func(t *testing.T) {
            result := parseSemver(tc.input)
            if result != tc.expected {
                t.Errorf("...")
            }
        })
    }
}
```

## Linter Configuration

### Current Exclusions (intentional)
```yaml
# internal/commands/help.go - help output is best-effort
# internal/commands/helpers.go - cmd.Help() stdout errors
# internal/updater/check.go - file/HTTP best-effort operations
# G204 (syscall.Exec) - intentional process replacement
```

### Adding New Exclusions
If you need to add exclusions:
1. Explain WHY in the exclusion comment
2. Keep it targeted (specific file/line, not broad)
3. Only for truly intentional cases

## File Permissions
Always use new octal format:
```go
// GOOD
os.WriteFile(path, data, 0o644)
os.MkdirAll(path, 0o755)

// BAD (old style)
os.WriteFile(path, data, 0644)
```

## Spellings
Use US English:
- `behavior` not `behaviour`
- `color` not `colour`
- `synchronize` not `synchronise`

## Common Pitfalls

### Variable Shadowing
Don't use `min`, `max`, `len` as variable names (shadow builtins). Use:
- `minArgs`, `maxArgs`
- `length` or `n`

### Unused Imports
Never leave unused imports. If refactoring removes the last use of a package, remove the import.

### Defer in Loops
Careful with `defer` in loops - it accumulates. Either:
- Wrap in function call
- Explicitly call close at end of iteration

## Security Context

The CLI uses `syscall.Exec()` to replace itself with ansible-playbook. This is:
1. **Intentional** - for signal handling
2. **Safe** - argv/env from trusted sources
3. **Documented** - see code comments

When working with subprocess code, maintain this security model.

## Documentation

If you add new features:
1. Update this AGENTS.md if it changes development patterns
2. Update README.md for user-facing changes
3. Update SKILLS.md for domain knowledge changes

## Questions?

If unsure about something:
1. Check existing code for patterns
2. Run the full verification suite
3. Look at the git history for similar changes
4. Ask the user if still unclear
