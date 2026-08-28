package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
)

//go:embed repomind-query repomind-summary repomind-init repomind-prd repomind-compact
var skillFiles embed.FS

func CompactPrompt() (string, error) {
	data, err := skillFiles.ReadFile("repomind-compact/SKILL.md")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func InstallSkills(repoRoot string) error {
	entries, err := skillFiles.ReadDir(".")
	if err != nil {
		return err
	}

	targets := []string{
		path.Join(repoRoot, ".claude", "skills"),
		path.Join(repoRoot, ".codex", "skills"),
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		for _, dstRoot := range targets {
			dstDir := path.Join(dstRoot, skillName)
			if err := replaceEmbeddedDir(skillFiles, skillName, dstDir); err != nil {
				return fmt.Errorf("replace %s: %w", dstDir, err)
			}
		}
	}
	return nil
}

// replaceEmbeddedDir installs one managed skill as an exact snapshot. Copying
// over an existing directory would leave files removed by newer releases.
func replaceEmbeddedDir(fsys fs.FS, srcDir, dstDir string) error {
	parent := path.Dir(dstDir)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}

	staging, err := os.MkdirTemp(parent, "."+path.Base(dstDir)+"-staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	if err := copyEmbeddedDir(fsys, srcDir, staging); err != nil {
		return err
	}

	backup := ""
	if _, err := os.Stat(dstDir); err == nil {
		backup, err = os.MkdirTemp(parent, "."+path.Base(dstDir)+"-backup-")
		if err != nil {
			return err
		}
		if err := os.Remove(backup); err != nil {
			return err
		}
		if err := os.Rename(dstDir, backup); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(staging, dstDir); err != nil {
		if backup != "" {
			_ = os.Rename(backup, dstDir)
		}
		return err
	}
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			return err
		}
	}
	return nil
}

func copyEmbeddedDir(fsys fs.FS, srcDir, dstDir string) error {
	return fs.WalkDir(fsys, srcDir, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(current, srcDir)
		rel = strings.TrimPrefix(rel, "/")
		target := dstDir
		if rel != "" {
			target = path.Join(dstDir, rel)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := fs.ReadFile(fsys, current)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(path.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}
