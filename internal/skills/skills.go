package skills

import (
	"embed"
	"io/fs"
	"os"
	"path"
	"strings"
)

//go:embed repomind-query repomind-summary repomind-init repomind-prd
var skillFiles embed.FS

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
			if err := copyEmbeddedDir(skillFiles, skillName, dstDir); err != nil {
				return err
			}
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
