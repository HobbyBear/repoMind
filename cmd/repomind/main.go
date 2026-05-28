package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"repomind/internal/cli"

	"github.com/spf13/cobra"
)

func main() {
	exeName := filepath.Base(os.Args[0])
	isInternal := strings.Contains(exeName, "repomind-internal")

	root := &cobra.Command{
		Use:   exeName,
		Short: "RepoMind - Skill-first business knowledge base for your code",
		Long: `RepoMind is a Skill-first local business code knowledge base system.

It is not a traditional CLI. The only command users need is:

  repomind install

Other commands are internal and used by RepoMind skills (Claude Code / Codex).`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	if isInternal {
		// repomind-internal mode: internal commands + uninstall + update
		for _, c := range cli.InternalCmds() {
			root.AddCommand(c)
		}
		root.AddCommand(cli.UninstallCmd())
		root.AddCommand(cli.UpdateCmd())
	} else {
		// repomind mode: install + uninstall + update + all internal commands
		root.AddCommand(cli.InstallCmd())
		root.AddCommand(cli.UninstallCmd())
		root.AddCommand(cli.UpdateCmd())
		for _, c := range cli.InternalCmds() {
			root.AddCommand(c)
		}
		// Also add install as internal variant
		root.AddCommand(cli.InternalInstallCmd())
	}

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
