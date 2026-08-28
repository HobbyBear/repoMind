package kb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditReportsDocumentMetricsWithoutProjectDocument(t *testing.T) {
	root := t.TempDir()
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
	if report.Documents != 1 {
		t.Fatalf("documents = %d", report.Documents)
	}
	if report.TotalBytes == 0 {
		t.Fatal("expected total bytes")
	}
	if report.Score != 100 {
		t.Fatalf("score = %d", report.Score)
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
