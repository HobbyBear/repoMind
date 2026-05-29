package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"repomind/internal/fsutil"
	"repomind/internal/gitutil"
	"repomind/internal/skills"

	"github.com/spf13/cobra"
)

func UpdateCmd() *cobra.Command {
	var fromURL string
	c := &cobra.Command{
		Use:   "update",
		Short: "Update the repomind binary and refresh project skills",
		Long: `Download and replace the current repomind binary, then refresh
skills and internal tools in the current directory (if repomind is installed here).

The --from URL should point to the release directory containing binaries named
repomind-<os>-<arch> (e.g. repomind update --from https://github.com/owner/repo/releases/latest/download).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(fromURL)
		},
	}
	c.Flags().StringVar(&fromURL, "from", "https://github.com/HobbyBear/repoMind/releases/latest/download", "Release URL")
	return c
}

func runUpdate(fromURL string) error {
	fromURL = strings.TrimRight(fromURL, "/")
	binaryName := fmt.Sprintf("repomind-%s-%s", runtime.GOOS, runtime.GOARCH)
	downloadURL := fromURL + "/" + binaryName

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine current binary path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("cannot resolve binary path: %w", err)
	}

	fmt.Printf("Downloading %s\n", downloadURL)
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "repomind-update-*")
	if err != nil {
		return fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("download failed: %w", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("chmod failed: %w", err)
	}

	if err := os.Rename(tmpPath, exePath); err != nil {
		// Fallback for cross-device rename: copy
		if err := copyFile(tmpPath, exePath); err != nil {
			return fmt.Errorf("failed to replace binary: %w", err)
		}
	}

	fmt.Printf("Updated: %s\n", exePath)

	// If current directory has repomind installed, refresh skills + tools.
	// The new binary (which we just replaced the old one with) embeds the
	// latest skill files, so this syncs them into the project.
	projectRoot, _ := os.Getwd()
	if fsutil.Exists(filepath.Join(projectRoot, ".repomind")) {
		if err := syncProject(projectRoot); err != nil {
			fmt.Printf("Skills refresh skipped: %v (re-run in project dir)\n", err)
		}
	} else {
		fmt.Println("Run 'repomind install' or re-run 'repomind update' in your project directories to refresh skills.")
	}

	return nil
}

// syncProject refreshes embedded skills and internal binary in a project.
// Does not touch modules, index.json, graph data, git config, or CLAUDE.md.
func syncProject(projectRoot string) error {
	repomindDir := filepath.Join(projectRoot, ".repomind")

	if err := skills.InstallSkills(projectRoot); err != nil {
		return fmt.Errorf("skills: %w", err)
	}
	fmt.Println("Refreshed skills")

	if err := fsutil.CopyExecutable(filepath.Join(repomindDir, "bin", "repomind-internal")); err != nil {
		return fmt.Errorf("internal binary: %w", err)
	}
	fmt.Println("Refreshed internal binary")

	if err := ensureRepomindGitignore(projectRoot); err != nil {
		return fmt.Errorf("repomind gitignore: %w", err)
	}
	if err := ensureGraphifyGitignore(projectRoot); err != nil {
		return fmt.Errorf("graphify gitignore: %w", err)
	}

	gitRoot, _ := gitutil.GitRoot()
	stageAll(gitRoot, projectRoot)

	fmt.Println("Sync complete. Modules and index.json were preserved.")
	return nil
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.OpenFile(dst, os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}
