package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertManagedBlockReplacesOnlyRepomindSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")

	original := strings.Join([]string{
		"# Existing",
		"",
		"keep this header",
		"",
		"<!-- repomind-start -->",
		"",
		"old repomind text",
		"",
		"<!-- repomind-end -->",
		"",
		"keep this footer",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := upsertManagedBlock(path, "<!-- repomind-start -->", "<!-- repomind-end -->", "new repomind text"); err != nil {
		t.Fatalf("upsertManagedBlock: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(data)

	if !strings.Contains(got, "keep this header") || !strings.Contains(got, "keep this footer") {
		t.Fatalf("unexpected removal of surrounding content:\n%s", got)
	}
	if strings.Contains(got, "old repomind text") {
		t.Fatalf("old managed block still present:\n%s", got)
	}
	if strings.Count(got, "<!-- repomind-start -->") != 1 || strings.Count(got, "<!-- repomind-end -->") != 1 {
		t.Fatalf("managed block markers not normalized:\n%s", got)
	}
	if !strings.Contains(got, "new repomind text") {
		t.Fatalf("new managed block missing:\n%s", got)
	}
}

func TestRepomindInstructionsRequireSummaryGateForCorrections(t *testing.T) {
	content := repomindInstructionContent()

	for _, want := range []string{
		"每次执行过 `repomind-query` 后，最终答复前都必须进入一次 `repomind-summary` 的 summary gate",
		"每次完成代码修改、生成文件、修复 bug 或跑完验证后，最终答复前也必须进入一次 `repomind-summary` 的 summary gate",
		"不能因为“只是写代码”就跳过 gate",
		"每次代码修改、生成文件、修复 bug 或跑完验证后，最终答复前必须触发一次 summary gate",
		"用户纠正 AI 或 RepoMind 的业务结论、模块判断或排查结论时",
		"用户纠正业务事实、模块归属、入口位置、排查根因或历史结论时",
		"用户明确要求沉淀知识时",
		"不直接修改生成目录",
		"按 `code_refs/name/description/keywords` 选择每种类型最相关的 1-3 篇",
		"不调用 RepoMind CLI、生成索引或外部图谱",
		"不能创建中转文件或描述成后台任务",
		"文件超过 8 KiB 必须继续精简，超过 12 KiB 必须立即拆分",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("repomind instructions missing %q:\n%s", want, content)
		}
	}
	for _, unwanted := range []string{"kb-metadata", "kb-validate", "kb-build", "graphify", ".query-findings.json", "project.md", "项目概览"} {
		if strings.Contains(content, unwanted) {
			t.Fatalf("repomind instructions still contain %q:\n%s", unwanted, content)
		}
	}
}
