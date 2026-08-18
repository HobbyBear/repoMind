package kb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditReportsOnboardingAndOversizedFiles(t *testing.T) {
	root := t.TempDir()
	mustWriteAuditFile(t, filepath.Join(root, ".repomind", "project.md"), `---
name: "聊天系统"
description: "面向用户的聊天系统，新人从这里了解主流程和模块边界。"
status: active
keywords: ["聊天", "新人"]
---
# 聊天系统
## 这是一个什么系统
为用户提供实时聊天。
## 主要能力
- 会话与消息。
## 业务边界
- 不负责支付清算。
`)
	mustWriteAuditFile(t, filepath.Join(root, ".repomind", "modules", "chat.md"), `---
name: "聊天模块"
description: "处理会话和消息。"
status: active
keywords: ["聊天", "消息", "会话"]
---
# 聊天模块
## 模块职责
处理聊天。
## 包含能力
- 收发消息。
## 技术入口
- internal/chat
`)
	report, err := Audit(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Documents != 2 {
		t.Fatalf("documents = %d", report.Documents)
	}
	if report.TotalBytes == 0 {
		t.Fatal("expected total bytes")
	}
	if len(report.NewcomerChecks) != 2 {
		t.Fatalf("newcomer checks = %d", len(report.NewcomerChecks))
	}
}

func mustWriteAuditFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
