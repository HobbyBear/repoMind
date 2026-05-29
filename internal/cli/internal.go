package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"repomind/internal/gitutil"
	"repomind/internal/graph"

	"github.com/spf13/cobra"
)

func InternalCmds() []*cobra.Command {
	return []*cobra.Command{
		graphScanCmd(),
		syncProjectCmd(),
	}
}

func graphScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "graph-scan",
		Short: "Run graph analysis scan (internal)",
		Long:  "Read graphify output from graphify-out/ and write summary to .repomind/graph/.\n\nThis is an internal command used by RepoMind skills.",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cannot determine current directory: %w", err)
			}
			if _, err := gitutil.GitRoot(); err != nil {
				return fmt.Errorf("not in a git repository: %w", err)
			}
			repomindDir := filepath.Join(projectRoot, ".repomind")
			graphDir := filepath.Join(repomindDir, "graph")
			summary, err := graph.GraphScan(projectRoot, graphDir)
			if err != nil {
				return err
			}
			if err := graph.WriteSummary(graphDir, summary); err != nil {
				return err
			}
			fmt.Printf("Graph scan complete. Mode: %s, Module candidates: %d\n", summary.Mode, len(summary.ModuleCandidates))
			return nil
		},
	}
}

func syncProjectCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "sync-project",
		Short:  "Sync skills, rules, and tools into the current project (internal)",
		Long:   "Refresh embedded skills, agent instructions, gitignore files, and internal binary.\n\nThis is an internal command used by repomind update.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cannot determine current directory: %w", err)
			}
			return syncProject(projectRoot)
		},
	}
}
