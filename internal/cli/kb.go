package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"repomind/internal/kb"

	"github.com/spf13/cobra"
)

func kbMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "kb-migrate",
		Short:  "Migrate RepoMind knowledge files to the current format",
		Long:   "Normalize RepoMind knowledge files, convert legacy central indexes to per-file metadata, and remove obsolete README/index files.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cannot determine current directory: %w", err)
			}
			result, err := kb.Migrate(projectRoot)
			if err != nil {
				return err
			}
			return writeJSON(result)
		},
	}
}

func kbMetadataCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "kb-metadata",
		Short:  "List RepoMind knowledge metadata for routing",
		Long:   "Scan .repomind/concepts, modules, and troubles and print each file's name and description metadata for skill-style routing.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cannot determine current directory: %w", err)
			}
			index, err := kb.BuildMetadata(projectRoot)
			if err != nil {
				return err
			}
			return writeJSON(index)
		},
	}
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
