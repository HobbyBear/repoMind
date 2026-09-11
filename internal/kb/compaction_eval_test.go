package kb

import (
	"path/filepath"
	"testing"
)

func TestCompareCompactionRewardsDeduplicationWithoutKnowledgeLoss(t *testing.T) {
	before := t.TempDir()
	after := t.TempDir()

	mustWriteFile(t, filepath.Join(before, ".repomind", "troubles", "payment-a.md"), `---
name: "支付成功订单未推进"
description: "支付完成但订单仍处理中。"
status: active
code_refs: ["example/internal/payment.HandleCallback"]
---

# 支付成功订单未推进

## 问题

支付完成但订单仍然处于处理中。

## 排查路径

- 先检查支付回调是否已经消费。
- 再检查订单状态机是否允许从 pending 进入 paid。

## 验证方式

- 回调消费成功后订单应进入 paid。

## 修订记录

- 2026-01-01：首次记录。
`)
	mustWriteFile(t, filepath.Join(before, ".repomind", "troubles", "payment-b.md"), `---
name: "支付回调后订单未更新"
description: "支付回调成功但订单状态没有变化。"
status: active
code_refs: ["example/internal/order.AdvanceState"]
---

# 支付回调后订单未更新

## 问题现象

支付完成但订单仍然处于处理中。

## 排查方法

- 先检查支付回调是否已经消费。
- 状态机拒绝 pending 到 paid 时检查重复回调和终态保护。

## 结果判断

- 回调已消费但状态未变化，说明状态机拒绝了转换。
`)
	mustWriteFile(t, filepath.Join(after, ".repomind", "troubles", "payment-pending.md"), `---
name: "支付完成但订单仍处理中"
description: "支付完成后订单未进入 paid 时查看，首查回调消费，再检查订单状态机。"
status: active
code_refs:
- "example/internal/payment.HandleCallback"
- "example/internal/order.AdvanceState"
---

# 支付完成但订单仍处理中

## 问题现象

支付完成但订单仍然处于处理中。

## 排查方法

1. 先检查支付回调是否已经消费。
2. 再检查订单状态机是否允许从 pending 进入 paid。
3. 状态机拒绝转换时检查重复回调和终态保护。

## 结果判断

- 回调消费成功且转换有效：订单应进入 paid。
- 回调已消费但状态未变化：状态机拒绝了转换。
`)

	comparison, err := CompareCompaction(before, after)
	if err != nil {
		t.Fatalf("CompareCompaction: %v", err)
	}
	if comparison.DocumentReduction != 1 || comparison.ByteReduction <= 0 {
		t.Fatalf("expected document and byte reduction: %#v", comparison)
	}
	if comparison.LexicalUnitRetention < 0.85 {
		t.Fatalf("unexpected retention: %#v", comparison)
	}
	if comparison.EvidenceAnchorRetention != 1 {
		t.Fatalf("evidence anchors were not retained: %#v", comparison)
	}
	if comparison.CodeRefRetention != 1 {
		t.Fatalf("code refs were not retained: %#v", comparison)
	}
	if comparison.After.DuplicateRatio >= comparison.Before.DuplicateRatio {
		t.Fatalf("duplicate ratio did not improve: %#v", comparison)
	}
	if comparison.QualityScoreDelta <= 0 || len(comparison.Risks) != 0 {
		t.Fatalf("quality did not improve cleanly: %#v", comparison)
	}
}

func TestCompareCompactionFlagsPotentialKnowledgeLoss(t *testing.T) {
	before := t.TempDir()
	after := t.TempDir()
	mustWriteFile(t, filepath.Join(before, ".repomind", "troubles", "source.md"), knowledgeFixture(
		"奖励未到账", "活动奖励显示成功但余额未增加。", []string{"奖励"},
		[]string{"example/internal/reward.Grant"}, "检查事件消费、账本流水和幂等记录。",
	))
	mustWriteFile(t, filepath.Join(after, ".repomind", "troubles", "source.md"), knowledgeFixture(
		"奖励未到账", "活动奖励显示成功但余额未增加。", []string{"奖励"}, nil,
		"只检查事件消费。",
	))

	comparison, err := CompareCompaction(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if comparison.CodeRefRetention != 0 || len(comparison.PotentiallyLostUnits) == 0 {
		t.Fatalf("expected loss evidence: %#v", comparison)
	}
	if !containsString(comparison.Risks, "code_refs_lost") {
		t.Fatalf("missing code_refs risk: %#v", comparison.Risks)
	}
}

func TestCompareCompactionTreatsIdentifierCaseStylesAsEquivalent(t *testing.T) {
	before := t.TempDir()
	after := t.TempDir()
	mustWriteFile(t, filepath.Join(before, ".repomind", "troubles", "source.md"), knowledgeFixture(
		"奖励进度异常", "奖励进度与实际发放时间不一致。", []string{"奖励"}, nil,
		"检查 `LastPaidAt` 和 `GrantedIndex` 是否来自同一期。",
	))
	mustWriteFile(t, filepath.Join(after, ".repomind", "troubles", "source.md"), knowledgeFixture(
		"奖励进度异常", "奖励进度与实际发放时间不一致。", []string{"奖励"}, nil,
		"检查 `last_paid_at` 和 `granted_index` 是否来自同一期。",
	))

	comparison, err := CompareCompaction(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if comparison.EvidenceAnchorRetention != 1 || len(comparison.PotentiallyLostAnchors) != 0 {
		t.Fatalf("identifier style caused false evidence loss: %#v", comparison)
	}
}
