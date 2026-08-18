package kb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateConvertsLegacyKnowledgeLayout(t *testing.T) {
	projectRoot := t.TempDir()
	repomindDir := filepath.Join(projectRoot, ".repomind")

	mustMkdir(t, filepath.Join(repomindDir, "modules"))
	mustMkdir(t, filepath.Join(repomindDir, "concepts"))
	mustMkdir(t, filepath.Join(repomindDir, "troubles"))

	mustWriteFile(t, filepath.Join(repomindDir, "index.json"), `{
  "modules": [
    {
      "file": "payment.md",
      "description": "支付核心模块，处理支付、退款、回调通知"
    },
    {
      "file": "order.md",
      "description": "订单核心模块，处理下单、状态流转和订单查询"
    }
  ]
}
`)
	mustWriteFile(t, filepath.Join(repomindDir, "modules", "README.md"), "# legacy modules readme\n")
	mustWriteFile(t, filepath.Join(repomindDir, "concepts", "README.md"), "# legacy concepts readme\n")
	mustWriteFile(t, filepath.Join(repomindDir, "troubles", "README.md"), "# legacy troubles readme\n")

	mustWriteFile(t, filepath.Join(repomindDir, "modules", "payment.md"), `# Payment

## 业务描述

支付与退款入口，覆盖 App 内交易、回调和补偿流程。
`)

	mustWriteFile(t, filepath.Join(repomindDir, "concepts", "pro-role.md"), `# 概念：Pro 角色

## 是什么

面向高价值用户的高级身份。

## 用户侧表现

用户可见专属权益和模型能力。
`)

	mustWriteFile(t, filepath.Join(repomindDir, "troubles", "vip-delay.md"), `# 排查：VIP 延迟生效

## 问题

VIP 购买后权益没有立即生效。

## 根因

缓存刷新延迟导致展示落后于账务状态。
`)

	result, err := Migrate(projectRoot)
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	if result.FormatVersion != CurrentFormatVersion {
		t.Fatalf("unexpected format version: %d", result.FormatVersion)
	}

	for _, removed := range []string{
		filepath.Join(repomindDir, "index.json"),
		filepath.Join(repomindDir, "modules", "README.md"),
		filepath.Join(repomindDir, "concepts", "README.md"),
		filepath.Join(repomindDir, "troubles", "README.md"),
	} {
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat err=%v", removed, err)
		}
	}

	assertContains(t, filepath.Join(repomindDir, ".kb-format.json"), `"version": 3`)
	assertContains(t, filepath.Join(repomindDir, "modules", "payment.md"), `name: "Payment"`)
	assertContains(t, filepath.Join(repomindDir, "modules", "payment.md"), `description: "支付核心模块，处理支付、退款、回调通知"`)
	assertContains(t, filepath.Join(repomindDir, "modules", "payment.md"), `keywords:`)
	assertContains(t, filepath.Join(repomindDir, "concepts", "pro-role.md"), `name: "Pro 角色"`)
	assertContains(t, filepath.Join(repomindDir, "troubles", "vip-delay.md"), `name: "VIP 延迟生效"`)
	assertContains(t, filepath.Join(repomindDir, "modules", "order.md"), `description: "订单核心模块，处理下单、状态流转和订单查询"`)
}

func TestBuildMetadataNormalizesManualDocumentAfterMigration(t *testing.T) {
	projectRoot := t.TempDir()
	repomindDir := filepath.Join(projectRoot, ".repomind")
	mustWriteFile(t, filepath.Join(repomindDir, ".kb-format.json"), `{"version": 3}`)
	mustWriteFile(t, filepath.Join(repomindDir, "concepts", "manual.md"), `# 人工新增概念

## 这是什么

运营同学直接新增的业务概念。

## 核心规则

- 仅用于测试。
`)

	index, err := BuildMetadata(projectRoot)
	if err != nil {
		t.Fatalf("BuildMetadata() error = %v", err)
	}
	if len(index.Concepts) != 1 || index.Concepts[0].Name != "人工新增概念" {
		t.Fatalf("unexpected concepts metadata: %#v", index.Concepts)
	}
	assertContains(t, filepath.Join(repomindDir, "concepts", "manual.md"), `name: "人工新增概念"`)
}

func TestSearchFindsKnowledgeAddedOnlyToBody(t *testing.T) {
	projectRoot := t.TempDir()
	repomindDir := filepath.Join(projectRoot, ".repomind")
	mustWriteFile(t, filepath.Join(repomindDir, "troubles", "payment.md"), `---
name: "支付异常"
description: "处理支付状态异常时查看。"
---

# 支付异常

## 问题现象

海外续费订单出现钻石重复发放。

## 排查方法

检查幂等键和回调记录。

## 验证方式

对比订单流水。
`)

	response, err := Search(projectRoot, SearchOptions{Query: "海外续费为什么钻石重复发放", Limit: 3})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(response.Results) == 0 || response.Results[0].File != "troubles/payment.md" {
		t.Fatalf("body knowledge was not retrieved: %#v", response.Results)
	}
	if !containsString(response.Results[0].MatchedFields, "body") {
		t.Fatalf("expected body match trace: %#v", response.Results[0])
	}
}

func TestValidateReportsKeywordAndCompactionLimits(t *testing.T) {
	projectRoot := t.TempDir()
	keywords := ""
	for i := 0; i < MaxKeywords+1; i++ {
		keywords += "- keyword" + string(rune('a'+i)) + "\n"
	}
	largeBody := strings.Repeat("排查内容。", HardFileBytes/3)
	mustWriteFile(t, filepath.Join(projectRoot, ".repomind", "troubles", "large.md"), `---
name: "大型排查文档"
description: "用于验证文档体积和关键词限制。"
keywords:
`+keywords+`---

# 大型排查文档

## 问题现象

异常。

## 排查方法

`+largeBody+`

## 验证方式

确认结果。
`)

	report, err := Validate(projectRoot)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if report.Errors == 0 || !hasIssue(report, "file_must_split") {
		t.Fatalf("expected hard split error: %#v", report)
	}
	if !hasIssue(report, "too_many_keywords") {
		t.Fatalf("expected keyword warning: %#v", report)
	}
}

func TestValidateFilesIgnoresUnrelatedLegacyDebt(t *testing.T) {
	projectRoot := t.TempDir()
	repomindDir := filepath.Join(projectRoot, ".repomind")
	mustWriteFile(t, filepath.Join(repomindDir, "modules", "legacy.md"), `---
name: "历史大模块"
description: "历史遗留模块。"
status: active
---

# 历史大模块

## 模块职责

`+strings.Repeat("历史内容。", HardFileBytes/3)+`

## 包含能力

- 历史能力。

## 技术入口

- legacy.go
`)
	mustWriteFile(t, filepath.Join(repomindDir, "concepts", "fresh.md"), `---
name: "新概念"
description: "用于验证增量文件可独立验收。"
status: active
keywords: ["新概念", "增量验收"]
---

# 新概念

## 这是什么

一个完整的新概念。

## 核心规则

- 保持独立校验。
`)

	report, err := ValidateFiles(projectRoot, []string{"concepts/fresh.md"})
	if err != nil {
		t.Fatalf("ValidateFiles() error = %v", err)
	}
	if !report.Valid || report.Errors != 0 || report.Warnings != 0 {
		t.Fatalf("unrelated legacy debt blocked incremental validation: %#v", report)
	}
}

func TestBuildCreatesEditableProjectAndGeneratedViews(t *testing.T) {
	projectRoot := t.TempDir()
	mustMkdir(t, filepath.Join(projectRoot, ".repomind", "concepts"))

	result, err := Build(projectRoot)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if result.Catalog != ".repomind/.generated/catalog.json" {
		t.Fatalf("unexpected catalog path: %#v", result)
	}
	assertContains(t, filepath.Join(projectRoot, ".repomind", "project.md"), "## 这是一个什么系统")
	assertContains(t, filepath.Join(projectRoot, ".repomind", "README.md"), "此页由 `repomind kb-build` 生成")
	assertContains(t, filepath.Join(projectRoot, ".repomind", ".generated", "catalog.json"), `"format_version": 3`)
}

func TestCreateUsesHumanEditableTemplate(t *testing.T) {
	projectRoot := t.TempDir()
	result, err := Create(projectRoot, CreateOptions{
		Kind: KindTrouble, Name: "充值订单处理中", Description: "充值订单长期处理中时查看。", Keywords: []string{"充值", "订单"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.File != ".repomind/troubles/充值订单处理中.md" {
		t.Fatalf("unexpected created file: %#v", result)
	}
	path := filepath.Join(projectRoot, filepath.FromSlash(result.File))
	assertContains(t, path, "## 问题现象")
	assertContains(t, path, "## 数据查询")
	assertContains(t, path, "## 结果判断")
	assertContains(t, path, "status: draft")
}

func TestSearchExcludesDraftByDefault(t *testing.T) {
	projectRoot := t.TempDir()
	_, err := Create(projectRoot, CreateOptions{Kind: KindConcept, Name: "草稿权益", Description: "尚未发布的权益说明。", Status: "draft"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	response, err := Search(projectRoot, SearchOptions{Query: "草稿权益"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(response.Results) != 0 {
		t.Fatalf("draft leaked into default search: %#v", response.Results)
	}

	response, err = Search(projectRoot, SearchOptions{Query: "草稿权益", IncludeDraft: true})
	if err != nil {
		t.Fatalf("Search(include draft) error = %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].Status != "draft" {
		t.Fatalf("draft was not returned explicitly: %#v", response.Results)
	}
}

func TestSearchExcludesDeprecatedByDefault(t *testing.T) {
	projectRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(projectRoot, ".repomind", "troubles", "old.md"), `---
name: "旧排查结论"
description: "已经作废的消息排查结论。"
status: deprecated
keywords: ["消息异常"]
---
# 旧排查结论
## 问题现象
消息异常。
## 排查方法
旧方法。
## 结果判断
已作废。
`)
	response, err := Search(projectRoot, SearchOptions{Query: "消息异常"})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 0 {
		t.Fatalf("deprecated leaked into default search: %#v", response.Results)
	}
	response, err = Search(projectRoot, SearchOptions{Query: "消息异常", IncludeDeprecated: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || response.Results[0].Status != "deprecated" {
		t.Fatalf("deprecated result = %#v", response.Results)
	}
}

func TestSearchPrioritizesProjectForOnboardingIntent(t *testing.T) {
	projectRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(projectRoot, ".repomind", "project.md"), `---
name: "聊天项目概览"
description: "新人了解聊天产品和阅读顺序的入口。"
status: active
---
# 聊天项目概览
## 这是一个什么系统
聊天产品。
## 主要能力
- 对话。
## 业务边界
- 不负责支付清算。
## 术语速查
- 会话：一组消息。
## 推荐阅读顺序
1. 先看聊天概念。
`)
	mustWriteFile(t, filepath.Join(projectRoot, ".repomind", "troubles", "noisy.md"), `---
name: "系统新人异常"
description: "新人遇到系统问题时排查。"
status: active
---
# 系统新人异常
## 问题现象
新人不知道系统是做什么的。
## 排查方法
查看系统项目概览。
## 结果判断
新人理解系统。
`)
	response, err := Search(projectRoot, SearchOptions{Query: "这个系统是做什么的，新人应该先看什么", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) == 0 || response.Results[0].File != "project.md" {
		t.Fatalf("onboarding results = %#v", response.Results)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasIssue(report ValidationReport, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestBuildMetadataReturnsPerFileRoutingEntries(t *testing.T) {
	projectRoot := t.TempDir()
	repomindDir := filepath.Join(projectRoot, ".repomind")

	mustMkdir(t, filepath.Join(repomindDir, "concepts"))
	mustMkdir(t, filepath.Join(repomindDir, "modules"))
	mustMkdir(t, filepath.Join(repomindDir, "troubles"))

	mustWriteFile(t, filepath.Join(repomindDir, "concepts", "pro-role.md"), `---
name: "Pro 角色"
description: "高级用户身份概念。用于判断权益范围、典型触发场景，以及和 VIP 的区别。"
---

# 概念：Pro 角色
`)

	mustWriteFile(t, filepath.Join(repomindDir, "modules", "payment.md"), `---
name: "支付模块"
description: "支付与退款相关模块。用于定位下单、回调、补偿入口和改动影响面。"
keywords:
- "支付"
- "payment"
- "refund"
---

# 支付模块
`)

	mustWriteFile(t, filepath.Join(repomindDir, "troubles", "vip-delay.md"), `---
name: "VIP 延迟生效"
description: "处理 VIP 购买后权益未及时生效时查看。包含首查方向和常见根因。"
---

# 排查：VIP 延迟生效
`)

	index, err := BuildMetadata(projectRoot)
	if err != nil {
		t.Fatalf("BuildMetadata() error = %v", err)
	}

	if len(index.Concepts) != 1 || index.Concepts[0].File != "concepts/pro-role.md" {
		t.Fatalf("unexpected concepts metadata: %#v", index.Concepts)
	}
	if len(index.Modules) != 1 || index.Modules[0].Name != "支付模块" {
		t.Fatalf("unexpected modules metadata: %#v", index.Modules)
	}
	if len(index.Modules[0].Keywords) != 3 || index.Modules[0].Keywords[0] != "支付" || index.Modules[0].Keywords[2] != "refund" {
		t.Fatalf("unexpected module keywords: %#v", index.Modules[0].Keywords)
	}
	if len(index.Troubles) != 1 || !strings.Contains(index.Troubles[0].Description, "常见根因") {
		t.Fatalf("unexpected troubles metadata: %#v", index.Troubles)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func assertContains(t *testing.T, path, needle string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if !strings.Contains(string(data), needle) {
		t.Fatalf("expected %s to contain %q, got:\n%s", path, needle, string(data))
	}
}
