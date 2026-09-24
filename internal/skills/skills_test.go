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
		filepath.Join(repoRoot, ".codex", "skills", "repomind-prd", "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected installed skill asset %s: %v", path, err)
		}
	}

	query, err := os.ReadFile(filepath.Join(repoRoot, ".codex", "skills", "repomind-query", "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed query skill: %v", err)
	}
	for _, want := range []string{"平台原生文件搜索", "code_refs", "每个激活的知识类型最多打开 1-3 篇正文", "直接执行 Summary Gate", "不得创建中转文件"} {
		if !strings.Contains(string(query), want) {
			t.Fatalf("query skill missing direct workflow rule %q", want)
		}
	}

	summary, err := os.ReadFile(filepath.Join(repoRoot, ".codex", "skills", "repomind-summary", "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed summary skill: %v", err)
	}
	for _, want := range []string{"`SKIP`", "`UPDATE`", "`MERGE`", "`CREATE`", "code_refs", "Trouble 二次准入", "裸日志", "低于 70/100 时丢弃", "## 判断分支", "文件超过 8 KiB", "超过 12 KiB 必须立即拆分", "至少有一个有效判断分支", "直接自检"} {
		if !strings.Contains(string(summary), want) {
			t.Fatalf("summary skill missing validation gate %q", want)
		}
	}

	compact, err := CompactPrompt()
	if err != nil {
		t.Fatalf("CompactPrompt: %v", err)
	}
	for _, want := range []string{
		"局部模式",
		"硬性 allowlist",
		"领域模式",
		"Trouble 整体治理模式",
		"两阶段流程",
		"用户症状族相同或高度重叠",
		"合并矩阵",
		"压缩前事实台账",
		"证据锚点至少保留 80%",
		"Trouble 事件升维与聚类",
		"稳定事实必须映射到",
		"最终有效结论裁决",
		"全部稳定事实均已映射",
		"首屏必须能完成症状确认和第一次分流",
		"`KEEP`",
		"`REPLACE`",
		"`MOVE`",
		"`MERGE`",
		"`SPLIT`",
		"`DISCARD`",
		"文件超过 12 KiB：必须立即拆分",
		"单章节超过 4 KiB：必须立即拆分",
		"标题层级保持正确",
		"只生成可审阅草案",
	} {
		if !strings.Contains(compact, want) {
			t.Fatalf("compact prompt missing %q", want)
		}
	}

	for _, skillName := range []string{"repomind-init", "repomind-query", "repomind-summary", "repomind-compact", "repomind-prd"} {
		data, err := os.ReadFile(filepath.Join(repoRoot, ".codex", "skills", skillName, "SKILL.md"))
		if err != nil {
			t.Fatalf("read installed %s skill: %v", skillName, err)
		}
		for _, unwanted := range []string{
			"project.md", "project_knowledge", "项目概览", "新人入口",
			"command -v repomind", "install.sh | bash", "Get-Command repomind", "install.ps1 | iex",
			"repomind kb-", "kb-metadata", "kb-build", "kb-validate", "kb-audit", "graphify", ".query-findings.json",
		} {
			if strings.Contains(string(data), unwanted) {
				t.Fatalf("%s skill still references %q", skillName, unwanted)
			}
		}
		for _, want := range []string{"平台原生文件搜索", "不依赖 RepoMind CLI"} {
			if !strings.Contains(string(data), want) {
				t.Fatalf("%s skill missing native workflow rule %q", skillName, want)
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
