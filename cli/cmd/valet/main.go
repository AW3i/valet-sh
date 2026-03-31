// Copyright 2025 TechDivision GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/valet-sh/valet-sh/cli/internal/commands"
)

// Version is set at build time via -ldflags "-X main.Version=x.y.z".
var Version = "dev"

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		// cobra already prints the error; just exit non-zero.
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "valet",
		Short: "valet.sh — local dev environment manager for Magento and PHP projects",
		Long: `valet.sh manages your local development environment for Magento, Neos,
AEM, and other PHP-based projects. It handles multiple simultaneous versions
of PHP, MySQL/MariaDB, Elasticsearch/OpenSearch, Redis, RabbitMQ, and nginx
on both Ubuntu and macOS (Intel and Apple Silicon).

Configuration is driven by a .valet-sh.yml file in each project directory.`,
		// Suppress the default "Error: unknown command" message — Ansible's
		// callback plugin handles its own output.
		SilenceUsage: true,
		Version:      Version,
	}

	// Print version in the same style as the rest of the tool.
	cmd.SetVersionTemplate(fmt.Sprintf("valet.sh %s\n", Version))

	// Register all subcommands.
	cmd.AddCommand(
		commands.NewInstallCmd(),
		commands.NewInitCmd(),
		commands.NewInitInstanceCmd(),
		commands.NewServiceCmd(),
		commands.NewLinkCmd(),
		commands.NewUnlinkCmd(),
		commands.NewLinksCmd(),
		commands.NewConfigCmd(),
		commands.NewDBCmd(),
		commands.NewExecCmd(),
		commands.NewRestoreCmd(),
		commands.NewXdebugCmd(),
		commands.NewPhpStormCmd(),
		commands.NewProjectCmd(),
		commands.NewUpdateDevCACmd(),
		commands.NewXPSSetupCmd(),
	)

	return cmd
}
