package skills

import (
	"os"
	"path/filepath"
	"strings"
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
		filepath.Join(repoRoot, ".claude", "skills", "repomind-compact", "SKILL.md"),
		filepath.Join(repoRoot, ".codex", "skills", "repomind-summary", "agents", "openai.yaml"),
		filepath.Join(repoRoot, ".codex", "skills", "repomind-compact", "SKILL.md"),
		filepath.Join(repoRoot, ".codex", "skills", "repomind-compact", "agents", "openai.yaml"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected installed skill asset %s: %v", path, err)
		}
	}

	query, err := os.ReadFile(filepath.Join(repoRoot, ".codex", "skills", "repomind-query", "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed query skill: %v", err)
	}
	if !strings.Contains(string(query), `repomind kb-search --query "<用户原始问题>"`) {
		t.Fatalf("query skill does not use RepoMind retrieval entry point")
	}

	summary, err := os.ReadFile(filepath.Join(repoRoot, ".codex", "skills", "repomind-summary", "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed summary skill: %v", err)
	}
	for _, want := range []string{"kb-validate", "--expect", "retrieval_queries"} {
		if !strings.Contains(string(summary), want) {
			t.Fatalf("summary skill missing retrieval gate %q", want)
		}
	}

	compact, err := CompactPrompt()
	if err != nil {
		t.Fatalf("CompactPrompt: %v", err)
	}
	for _, want := range []string{"整库精简", "kb-audit", "只生成可审阅草案"} {
		if !strings.Contains(compact, want) {
			t.Fatalf("compact prompt missing %q", want)
		}
	}
}
