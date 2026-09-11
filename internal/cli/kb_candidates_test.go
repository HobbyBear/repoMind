package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"repomind/internal/kb"
)

func TestKBMetadataQueryReturnsRankedCandidatesWithoutEditingKnowledge(t *testing.T) {
	root := t.TempDir()
	docPath := filepath.Join(root, ".repomind", "modules", "payment.md")
	content := `---
name: "支付回调模块"
description: "处理支付成功后的订单状态推进。"
status: active
keywords: ["支付", "回调"]
code_refs: ["example/internal/payment.(*Service).HandleCallback"]
---

# 支付回调模块

## 模块职责

处理支付回调。
`
	if err := os.MkdirAll(filepath.Dir(docPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	cmd := kbMetadataCmd()
	cmd.SetArgs([]string{"--query", "HandleCallback 后订单未更新", "--kind", "module", "--limit", "3"})
	execErr := cmd.Execute()
	_ = w.Close()
	os.Stdout = oldStdout
	output, readErr := io.ReadAll(r)
	_ = r.Close()
	if execErr != nil {
		t.Fatalf("execute kb-metadata: %v", execErr)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}

	var response kb.CandidateResponse
	if err := json.Unmarshal(output, &response); err != nil {
		t.Fatalf("decode output %q: %v", output, err)
	}
	if len(response.Candidates) != 1 || response.Candidates[0].File != "modules/payment.md" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if response.Candidates[0].MatchedFields[0] != "code_refs" {
		t.Fatalf("expected code_refs evidence first: %#v", response.Candidates[0])
	}
	after, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != content {
		t.Fatal("candidate query modified the knowledge document")
	}
}

func TestKBAuditCompareToReportsCompactionMetrics(t *testing.T) {
	before := t.TempDir()
	after := t.TempDir()
	beforeDoc := filepath.Join(before, ".repomind", "troubles", "payment.md")
	afterDoc := filepath.Join(after, ".repomind", "troubles", "payment.md")
	for _, dir := range []string{filepath.Dir(beforeDoc), filepath.Dir(afterDoc)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	frontMatter := `---
name: "支付回调异常"
description: "支付完成但订单仍然处于处理中。"
status: active
code_refs: ["example/internal/payment.HandleCallback"]
---

# 支付回调异常

## 问题现象

支付完成但订单仍然处于处理中。

## 排查方法

检查支付回调是否消费。

## 结果判断

回调未消费时重试消息。
`
	if err := os.WriteFile(beforeDoc, []byte(frontMatter+"\n## 修订记录\n\n- 首次排查。\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(afterDoc, []byte(frontMatter), 0644); err != nil {
		t.Fatal(err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(after); err != nil {
		t.Fatal(err)
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	cmd := kbAuditCmd()
	cmd.SetArgs([]string{"--compare-to", filepath.Join(before, ".repomind")})
	execErr := cmd.Execute()
	_ = w.Close()
	os.Stdout = oldStdout
	output, readErr := io.ReadAll(r)
	_ = r.Close()
	if execErr != nil {
		t.Fatalf("execute kb-audit --compare-to: %v", execErr)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}

	var comparison kb.CompactionComparison
	if err := json.Unmarshal(output, &comparison); err != nil {
		t.Fatalf("decode output %q: %v", output, err)
	}
	if comparison.Before.Documents != 1 || comparison.After.Documents != 1 {
		t.Fatalf("unexpected document counts: %#v", comparison)
	}
	if comparison.ByteReduction <= 0 || comparison.CodeRefRetention != 1 {
		t.Fatalf("unexpected comparison: %#v", comparison)
	}
}
