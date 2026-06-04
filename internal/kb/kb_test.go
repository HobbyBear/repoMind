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

	assertContains(t, filepath.Join(repomindDir, ".kb-format.json"), `"version": 2`)
	assertContains(t, filepath.Join(repomindDir, "modules", "payment.md"), `name: "Payment"`)
	assertContains(t, filepath.Join(repomindDir, "modules", "payment.md"), `description: "支付核心模块，处理支付、退款、回调通知"`)
	assertContains(t, filepath.Join(repomindDir, "modules", "payment.md"), `keywords:`)
	assertContains(t, filepath.Join(repomindDir, "concepts", "pro-role.md"), `name: "Pro 角色"`)
	assertContains(t, filepath.Join(repomindDir, "troubles", "vip-delay.md"), `name: "VIP 延迟生效"`)
	assertContains(t, filepath.Join(repomindDir, "modules", "order.md"), `description: "订单核心模块，处理下单、状态流转和订单查询"`)
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
	if len(index.Modules[0].Keywords) != 4 || index.Modules[0].Keywords[0] != "支付" || index.Modules[0].Keywords[3] != "支付模块" {
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
