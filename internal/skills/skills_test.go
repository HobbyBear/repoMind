package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSkillsCopiesEntireSkillDirectory(t *testing.T) {
	repoRoot := t.TempDir()

	if err := InstallSkills(repoRoot); err != nil {
		t.Fatalf("InstallSkills: %v", err)
	}

	for _, path := range []string{
		filepath.Join(repoRoot, ".claude", "skills", "repomind-query", "SKILL.md"),
		filepath.Join(repoRoot, ".claude", "skills", "repomind-query", "agents", "openai.yaml"),
		filepath.Join(repoRoot, ".codex", "skills", "repomind-summary", "agents", "openai.yaml"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected installed skill asset %s: %v", path, err)
		}
	}
}
