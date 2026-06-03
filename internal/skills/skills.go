package skills

import (
	"embed"
	"os"
	"path"
)

//go:embed repomind-query/SKILL.md repomind-summary/SKILL.md repomind-init/SKILL.md
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
		data, err := skillFiles.ReadFile(path.Join(skillName, "SKILL.md"))
		if err != nil {
			return err
		}
		for _, dstRoot := range targets {
			dstDir := path.Join(dstRoot, skillName)
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				return err
			}
			if err := os.WriteFile(path.Join(dstDir, "SKILL.md"), data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
