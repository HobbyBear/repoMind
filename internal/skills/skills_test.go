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
	if !strings.Contains(string(query), "先只读取每篇文档开头的 YAML frontmatter") {
		t.Fatalf("query skill does not route directly from frontmatter")
	}
	for _, unwanted := range []string{"kb-build", "kb-search"} {
		if strings.Contains(string(query), unwanted) {
			t.Fatalf("query skill still references %q", unwanted)
		}
	}

	summary, err := os.ReadFile(filepath.Join(repoRoot, ".codex", "skills", "repomind-summary", "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed summary skill: %v", err)
	}
	for _, want := range []string{"kb-validate", "--strict", "本次写入文件"} {
		if !strings.Contains(string(summary), want) {
			t.Fatalf("summary skill missing validation gate %q", want)
		}
	}
	for _, unwanted := range []string{"kb-build", "kb-search", "--expect"} {
		if strings.Contains(string(summary), unwanted) {
			t.Fatalf("summary skill still references %q", unwanted)
		}
	}

	compact, err := CompactPrompt()
	if err != nil {
		t.Fatalf("CompactPrompt: %v", err)
	}
	for _, want := range []string{"局部模式", "硬性 allowlist", "整库模式", "不要执行 `kb-build`", "只生成可审阅草案"} {
		if !strings.Contains(compact, want) {
			t.Fatalf("compact prompt missing %q", want)
		}
	}

	for _, skillName := range []string{"repomind-init", "repomind-query", "repomind-summary", "repomind-compact"} {
		data, err := os.ReadFile(filepath.Join(repoRoot, ".codex", "skills", skillName, "SKILL.md"))
		if err != nil {
			t.Fatalf("read installed %s skill: %v", skillName, err)
		}
		for _, unwanted := range []string{"project.md", "project_knowledge", "项目概览", "新人入口"} {
			if strings.Contains(string(data), unwanted) {
				t.Fatalf("%s skill still references %q", skillName, unwanted)
			}
		}
	}
}

func TestInstallSkillsReplacesOldSkillDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	if err := InstallSkills(repoRoot); err != nil {
		t.Fatalf("initial InstallSkills: %v", err)
	}

	for _, platform := range []string{".claude", ".codex"} {
		skillDir := filepath.Join(repoRoot, platform, "skills", "repomind-compact")
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("old skill"), 0644); err != nil {
			t.Fatalf("replace installed skill: %v", err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "obsolete.txt"), []byte("stale"), 0644); err != nil {
			t.Fatalf("write stale skill file: %v", err)
		}
	}

	if err := InstallSkills(repoRoot); err != nil {
		t.Fatalf("refresh InstallSkills: %v", err)
	}

	for _, platform := range []string{".claude", ".codex"} {
		skillDir := filepath.Join(repoRoot, platform, "skills", "repomind-compact")
		data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
		if err != nil {
			t.Fatalf("read refreshed skill: %v", err)
		}
		if !strings.Contains(string(data), "局部模式") || strings.Contains(string(data), "old skill") {
			t.Fatalf("skill was not refreshed from the embedded version: %s", data)
		}
		if _, err := os.Stat(filepath.Join(skillDir, "obsolete.txt")); !os.IsNotExist(err) {
			t.Fatalf("obsolete skill file was not removed: %v", err)
		}
	}
}
