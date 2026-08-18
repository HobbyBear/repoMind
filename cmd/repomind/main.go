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

Install RepoMind into a project with:

  repomind install

Knowledge commands such as kb-search, kb-build, kb-validate, and kb-new form
the stable data-source interface used by external systems and RepoMind skills.`,
		SilenceUsage:      true,
		SilenceErrors:     true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
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
