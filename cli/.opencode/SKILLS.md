# SKILLS.md - Domain Knowledge for valet-sh CLI

## Project Context

valet-sh is a local development environment manager for Magento, PHP, and other projects. It manages multiple simultaneous versions of:
- PHP (5.6 through 8.5)
- MariaDB/MySQL (multiple versions)
- Elasticsearch/OpenSearch
- Redis/Valkey
- RabbitMQ
- Nginx
- dnsmasq

## Architecture Overview

### Three-Layer Architecture

```
User
  ↓ runs `valet.sh init-instance`
valet-sh CLI (Go binary) ← THIS CODEBASE
  ↓ reads .valet-sh.yml
  ↓ execs ansible-playbook
Ansible Playbooks
  ↓ manages services
System (macOS/Ubuntu)
```

### Why Go + Ansible?

The original tool was pure Ansible. The Go CLI was added to:
1. Provide typed config validation (YAML → Go structs)
2. Better UX (help text, argument validation)
3. Update checking
4. Eventually replace the bash wrapper

**Important**: Go doesn't replace Ansible - it orchestrates it. All heavy lifting (installing packages, configuring nginx, etc.) still happens in Ansible.

## .valet-sh.yml Format

This is the project's "interface" - users define their needs here.

```yaml
# Remote hub for valet-restore command
hub:
  host: "git.example.com"
  port: 22
  path: "/data"

# Service versions required
services:
  php:
    version: 8.1
  mariadb:
    version: 10.6
    database: magento_prod_copy
  elasticsearch:
    version: 7
    plugins: ["analysis-icu"]
  redis: {}  # Just needs to exist

# Project metadata
instance:
  key: "myproject"        # Hostname: myproject.test
  type: "magento2"        # Bootstrap workflow
  path: "src"             # Docroot
  
  # Multi-domain support
  multidomain:
    "de.magento.test": "de_DE"
    
  # Sync configuration for valet-restore
  sync:
    identifier: "staging"
    db: true
    fs: ["pub/media", "var/log"]
```

### Instance Types
- `magento2` - Full Magento 2 workflow (env.php generation, indexer, etc.)
- `magento1` - Magento 1 workflow
- `neos` - Neos CMS workflow
- `aem` - Adobe Experience Manager
- `orocrm` - OroCRM

### Service Fuzzy Aliases

The CLI normalizes service names:
- `PHP8.3`, `php8.3`, `PHP83` → `php83`
- `mariadb10.4`, `MARIADB10.4` → `mariadb104`
- `mysql5.7`, `MYSQL5.7` → `mysql57`

See `platform.NormalizeServiceName()` for full mapping.

## File Locations

The tool installs to `/usr/local/valet-sh/` with this structure:

```
/usr/local/valet-sh/
├── bin/
│   └── valet                    # Go CLI binary (from this repo)
├── etc/
│   ├── config.yml               # Global config
│   ├── links.yml                # Symlink tracking
│   └── .last_update_check       # Update check timestamp
├── installer/
│   └── valet-sh-installer       # Separate Go binary (different repo)
├── packages/
│   └── *.tar.gz                 # Python venv, Homebrew packages
├── valet-sh/
│   ├── playbooks/               # Ansible playbooks (from this repo)
│   └── roles/                   # Ansible roles (from this repo)
└── venv/
    └── bin/
        ├── ansible-playbook     # From venv tarball
        └── valet.sh             # Bash wrapper (delegates to Go CLI)
```

## Update Flow

1. Go CLI has `updater.Check()` which runs at most once per week
2. Fetches `https://api.github.com/repos/valet-sh/valet-sh/releases/latest`
3. Compares version with baked-in `Version` var
4. If newer, prompts user: "Update now? [y/N]"
5. On yes: runs `valet-sh-installer update` then re-execs current command

## Release Process

GitHub Actions (`.github/workflows/release-cli.yml`):

```
git tag v2.10.0
  ↓
GitHub Actions triggers
  ↓
Builds 4 binaries:
  - valet-linux-amd64
  - valet-linux-arm64  
  - valet-darwin-amd64
  - valet-darwin-arm64
  ↓
Uploads to GitHub Release
  ↓
Installer downloads appropriate binary
```

## Security Model

### Subprocess Execution
The CLI uses `syscall.Exec()` to replace itself with ansible-playbook:
- **Why**: Signal handling (Ctrl-C flows directly to Ansible)
- **Safe**: argv constructed from trusted constants + user input
- **Same**: Same security model as original bash wrapper

### File Paths
- All paths are absolute (under `/usr/local/valet-sh/`)
- No user-controlled path traversal
- Downloads from GitHub releases (HTTPS + checksums in future)

## Common Operations

### Adding a New Command

Example: Adding `valet.sh backup` command

1. Create `internal/commands/backup.go`:
```go
package commands

import (
    "github.com/spf13/cobra"
    "github.com/valet-sh/valet-sh/cli/internal/ansible"
)

func NewBackupCmd() *cobra.Command {
    var verbose bool
    cmd := &cobra.Command{
        Use:   "backup [name]",
        Short: "Backup project database and files",
        Args:  requireMinArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return ansible.Run(&ansible.RunOpts{
                Playbook: "backup",
                Args:     args,
                Verbose:  verbose,
            })
        },
    }
    cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
    return cmd
}
```

2. Add to `cmd/valet/main.go`:
```go
cmd.AddCommand(
    // ... existing commands ...
    commands.NewBackupCmd(),
)
```

3. Create `playbooks/backup.yml` (Ansible, separate concern)

### Adding Config Validation

In `internal/config/project.go`, add to `Validate()`:
```go
if c.Services.Redis != nil && c.Services.Valkey != nil {
    errs = append(errs, "services.redis and services.valkey cannot both be set")
}
```

## Testing Strategies

### Unit Tests
Test pure Go logic (parsing, validation, etc.)
- Example: `updater/check_test.go` tests semver comparison
- No external dependencies
- Fast execution

### Integration Tests
Not currently implemented, but would test:
- Temp .valet-sh.yml file → parse → validate workflow
- Mock ansible-playbook binary
- End-to-end command execution

## Platform Differences

### macOS vs Linux
The Go CLI doesn't handle platform differences - Ansible does.

Go just detects platform (`platform.Detect()`) and passes to Ansible via extra-vars:
- `valet_current_path` - working directory
- Ansible roles handle OS-specific logic

### Intel vs ARM (Apple Silicon)
Detected via `runtime.GOARCH`:
- `amd64` → Intel
- `arm64` → Apple Silicon / ARM64

## Version Handling

Versions are stored as:
- Git tags: `v2.10.0` (with 'v' prefix)
- In-code: `2.10.0` (without 'v')
- Git describe: `2.9.19-102-g35e11d2` (for dev builds)

Comparison uses semver parsing (major, minor, patch), ignoring git suffixes.

## Future Enhancements

### Checksum Verification (In Progress)
Planned: Installer should verify binary checksums against checksums.txt from releases.

### Self-Update (In Progress)
Planned: Go CLI handles its own updates instead of calling installer.

### More Commands
Potential additions:
- `valet.sh doctor` - diagnostic/health check
- `valet.sh logs` - service log aggregation
- `valet.sh snapshot` - project state snapshots

## Troubleshooting

### "ansible-playbook not found"
Check: `/usr/local/valet-sh/venv/bin/ansible-playbook` exists
Fix: Run `valet-sh-installer setup` to recreate venv

### Update check not running
Check: `/usr/local/valet-sh/etc/.last_update_check` timestamp
If less than 7 days old, update check is skipped (by design)

### Linter errors
See AGENTS.md - do NOT add nolint comments.

## Domain Terminology

- **valet-sh** - The project name
- **valet.sh** - The user-facing command (symlink to Go binary via bash wrapper)
- **init-instance** - Bootstrap a project from .valet-sh.yml
- **link** - Create nginx vhost for current directory
- **restore** - Sync DB/files from remote hub
- **service** - Manage background services (start/stop/restart)

## External Dependencies

### GitHub Repositories
- `valet-sh/valet-sh` - This repo (Go CLI + Ansible)
- `valet-sh/installer` - Bootstrap installer (separate Go binary)
- `valet-sh/cli` - Python package with bash wrapper (separate repo)
- `valet-sh/runtime` - Python venv tarball (separate repo)

### Binary Dependencies (managed by installer)
- Go 1.22+
- Python 3.x + venv
- Ansible (via pip in venv)
- Various system packages (nginx, dnsmasq, etc.)

## License & Copyright

Apache 2.0 - TechDivision GmbH 2025
All code files must include the standard copyright header.
