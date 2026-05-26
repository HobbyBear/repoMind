package cli

import (
	"fmt"
	"path/filepath"

	"repomind/internal/gitutil"
	"repomind/internal/graph"

	"github.com/spf13/cobra"
)

func InternalCmds() []*cobra.Command {
	return []*cobra.Command{
		graphScanCmd(),
	}
}

func graphScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "graph-scan",
		Short: "Run graph analysis scan (internal)",
		Long:  "Read graphify output from graphify-out/ and write summary to .repomind/graph/.\n\nThis is an internal command used by RepoMind skills.",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := gitutil.GitRoot()
			if err != nil {
				return fmt.Errorf("not in a git repository: %w", err)
			}
			repomindDir := filepath.Join(repoRoot, ".repomind")
			graphDir := filepath.Join(repomindDir, "graph")
			summary, err := graph.GraphScan(repoRoot, graphDir)
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
