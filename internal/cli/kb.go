package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"repomind/internal/kb"

	"github.com/spf13/cobra"
)

func kbMigrateCmd() *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:    "kb-migrate",
		Short:  "Migrate RepoMind knowledge files to the current format",
		Long:   "Normalize RepoMind knowledge files, convert legacy central indexes to per-file metadata, and remove obsolete README/index files.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cannot determine current directory: %w", err)
			}
			var result *kb.MigrationResult
			if force {
				result, err = kb.ForceMigrate(projectRoot)
			} else {
				result, err = kb.Migrate(projectRoot)
			}
			if err != nil {
				return err
			}
			return writeJSON(result)
		},
	}
	c.Flags().BoolVar(&force, "force", false, "force normalization even when the knowledge base is already on the current format")
	return c
}

func kbMetadataCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kb-metadata",
		Short: "List RepoMind knowledge metadata for routing",
		Long:  "Scan .repomind/concepts, modules, and troubles and print each file's name and description metadata for skill-style routing.",
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

func kbBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kb-build",
		Short: "Build the RepoMind catalog and human-readable overview",
		Long:  "Normalize authored Markdown, validate it, and generate .repomind/.generated/catalog.json plus .repomind/README.md.",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cannot determine current directory: %w", err)
			}
			result, err := kb.Build(projectRoot)
			if err != nil {
				return err
			}
			return writeJSON(result)
		},
	}
}

func kbAuditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kb-audit",
		Short: "Audit whole knowledge-base readability",
		Long:  "Measure onboarding quality, routing metadata, document size, and newcomer retrieval for a whole RepoMind knowledge base.",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cannot determine current directory: %w", err)
			}
			report, err := kb.Audit(projectRoot)
			if err != nil {
				return err
			}
			return writeJSON(report)
		},
	}
}

func kbValidateCmd() *cobra.Command {
	var strict bool
	var files []string
	c := &cobra.Command{
		Use:   "kb-validate",
		Short: "Validate RepoMind knowledge files",
		Long:  "Check metadata, required sections, keyword limits, and document sizes. With --strict, warnings also fail the command.",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cannot determine current directory: %w", err)
			}
			report, err := kb.ValidateFiles(projectRoot, files)
			if err != nil {
				return err
			}
			if err := writeJSON(report); err != nil {
				return err
			}
			if report.Errors > 0 || strict && report.Warnings > 0 {
				return fmt.Errorf("knowledge validation failed: %d errors, %d warnings", report.Errors, report.Warnings)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&strict, "strict", false, "treat warnings as validation failures")
	c.Flags().StringSliceVar(&files, "file", nil, "validate only these .repomind-relative knowledge files")
	return c
}

func kbSearchCmd() *cobra.Command {
	var query, kindValue, expected string
	var limit int
	var includeDraft, includeDeprecated bool
	c := &cobra.Command{
		Use:   "kb-search",
		Short: "Search RepoMind metadata and document sections",
		Long:  "Return ranked knowledge candidates with matched fields and section snippets. External systems and repomind-query should use this as the retrieval entry point.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clean := strings.TrimSpace(query); clean == "" {
				return fmt.Errorf("--query is required")
			}
			kind := kb.Kind(strings.TrimSpace(kindValue))
			if kind != "" && kind != kb.KindProject && kind != kb.KindConcept && kind != kb.KindModule && kind != kb.KindTrouble {
				return fmt.Errorf("unsupported --kind %q", kindValue)
			}
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cannot determine current directory: %w", err)
			}
			response, err := kb.Search(projectRoot, kb.SearchOptions{Query: query, Kind: kind, Limit: limit, IncludeDraft: includeDraft, IncludeDeprecated: includeDeprecated})
			if err != nil {
				return err
			}
			if err := writeJSON(response); err != nil {
				return err
			}
			if expected != "" && !containsSearchFile(response, expected) {
				return fmt.Errorf("retrieval regression: expected %s in top %d results", expected, limit)
			}
			return nil
		},
	}
	c.Flags().StringVar(&query, "query", "", "the original user question or search phrase")
	c.Flags().StringVar(&kindValue, "kind", "", "optional kind filter: project, concept, module, trouble")
	c.Flags().IntVar(&limit, "limit", 5, "maximum number of results")
	c.Flags().StringVar(&expected, "expect", "", "fail unless this .repomind-relative file appears in the results")
	c.Flags().BoolVar(&includeDraft, "include-draft", false, "include draft knowledge in search results")
	c.Flags().BoolVar(&includeDeprecated, "include-deprecated", false, "include deprecated knowledge in search results")
	return c
}

func containsSearchFile(response *kb.SearchResponse, expected string) bool {
	expected = filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(expected), ".repomind/"))
	for _, result := range response.Results {
		if filepath.ToSlash(result.File) == expected {
			return true
		}
	}
	return false
}

func kbNewCmd() *cobra.Command {
	var kindValue, name, description, file, status string
	var keywords []string
	c := &cobra.Command{
		Use:   "kb-new",
		Short: "Create a human-editable knowledge document from a template",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cannot determine current directory: %w", err)
			}
			result, err := kb.Create(projectRoot, kb.CreateOptions{
				Kind: kb.Kind(kindValue), Name: name, Description: description, Keywords: keywords, File: file, Status: status,
			})
			if err != nil {
				return err
			}
			return writeJSON(result)
		},
	}
	c.Flags().StringVar(&kindValue, "kind", "", "knowledge kind: concept, module, or trouble")
	c.Flags().StringVar(&name, "name", "", "human-readable knowledge name")
	c.Flags().StringVar(&description, "description", "", "one or two sentences used for routing")
	c.Flags().StringSliceVar(&keywords, "keywords", nil, "3-8 search terms or aliases")
	c.Flags().StringVar(&file, "file", "", "optional .md filename")
	c.Flags().StringVar(&status, "status", "draft", "knowledge status: draft or active")
	_ = c.MarkFlagRequired("kind")
	_ = c.MarkFlagRequired("name")
	return c
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
