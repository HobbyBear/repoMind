package kb

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"
)

type recallCase struct {
	name      string
	query     string
	similarTo string
	kind      Kind
	want      string
}

type recallMetrics struct {
	top1    float64
	recall3 float64
	mrr     float64
}

func TestCandidateRecallQualityAgainstLegacyLexicalBaseline(t *testing.T) {
	projectRoot := candidateFixture(t)
	cases := candidateRecallCases()

	baseline := evaluateLegacyRecall(t, projectRoot, cases)
	optimized := evaluateCandidateRecall(t, projectRoot, cases)
	t.Logf("baseline  Top-1=%.1f%% Recall@3=%.1f%% MRR=%.3f", baseline.top1*100, baseline.recall3*100, baseline.mrr)
	t.Logf("optimized Top-1=%.1f%% Recall@3=%.1f%% MRR=%.3f", optimized.top1*100, optimized.recall3*100, optimized.mrr)

	if optimized.top1 < baseline.top1+0.20 {
		t.Fatalf("Top-1 improvement is too small: baseline=%.3f optimized=%.3f", baseline.top1, optimized.top1)
	}
	if optimized.recall3 < 1.0 {
		t.Fatalf("optimized Recall@3 = %.3f, want 1.0", optimized.recall3)
	}
	if optimized.mrr < 0.95 {
		t.Fatalf("optimized MRR = %.3f, want >= 0.95", optimized.mrr)
	}
}

func TestFindCandidatesReturnsStableSymbolEvidence(t *testing.T) {
	projectRoot := candidateFixture(t)
	response, err := FindCandidates(projectRoot, CandidateOptions{
		Query: "RefreshEntitlementCache 执行后会员权益仍未到账", Kind: KindTrouble, Limit: 3,
	})
	if err != nil {
		t.Fatalf("FindCandidates: %v", err)
	}
	if len(response.Candidates) == 0 || response.Candidates[0].File != "troubles/vip-cache-delay.md" {
		t.Fatalf("unexpected candidates: %#v", response.Candidates)
	}
	if !containsString(response.Candidates[0].MatchedFields, "code_refs") {
		t.Fatalf("missing code_refs evidence: %#v", response.Candidates[0])
	}
}

func TestFindSimilarCandidatesDefaultsToSeedKindAndExcludesSeed(t *testing.T) {
	projectRoot := candidateFixture(t)
	response, err := FindCandidates(projectRoot, CandidateOptions{
		SimilarTo: ".repomind/troubles/vip-cache-delay.md", Limit: 3,
	})
	if err != nil {
		t.Fatalf("FindCandidates: %v", err)
	}
	if response.Mode != "similar" || response.SeedFile != "troubles/vip-cache-delay.md" {
		t.Fatalf("unexpected response metadata: %#v", response)
	}
	if response.TotalMatches < response.Returned || response.Returned != len(response.Candidates) {
		t.Fatalf("unexpected candidate counts: %#v", response)
	}
	if len(response.Candidates) == 0 || response.Candidates[0].File != "troubles/vip-cache-stale-display.md" {
		t.Fatalf("unexpected similar candidates: %#v", response.Candidates)
	}
	for _, candidate := range response.Candidates {
		if candidate.Kind != KindTrouble || candidate.File == response.SeedFile {
			t.Fatalf("seed/kind isolation failed: %#v", candidate)
		}
	}
}

func TestFindCandidatesScansNestedDocumentsAndIgnoresSymlinks(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, ".repomind", "troubles", "payment", "callback.md")
	if err := os.MkdirAll(filepath.Dir(nested), 0755); err != nil {
		t.Fatal(err)
	}
	content := knowledgeFixture(
		"支付回调未推进", "支付完成但订单未更新。", []string{"支付回调"},
		[]string{"example/internal/payment.HandleCallback"}, "检查回调消费。",
	)
	if err := os.WriteFile(nested, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(root, "outside.md")
	if err := os.WriteFile(external, []byte(knowledgeFixture(
		"外部秘密", "不应被知识扫描读取。", []string{"external-secret"}, nil, "外部内容。",
	)), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, ".repomind", "troubles", "outside.md")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	response, err := FindCandidates(root, CandidateOptions{Query: "HandleCallback 订单未更新", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if candidateRank(response.Candidates, "troubles/payment/callback.md") != 1 {
		t.Fatalf("nested candidate not ranked first: %#v", response.Candidates)
	}
	for _, candidate := range response.Candidates {
		if candidate.File == "troubles/outside.md" {
			t.Fatalf("symlink target leaked into candidates: %#v", candidate)
		}
	}
}

func BenchmarkCandidateRanking(b *testing.B) {
	projectRoot := candidateFixture(b)
	addSyntheticCandidateDocs(b, projectRoot, 500)
	options := CandidateOptions{Query: "支付回调成功但订单仍在处理中 HandlePaymentCallback", Limit: 5}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := FindCandidates(projectRoot, options); err != nil {
			b.Fatal(err)
		}
	}
}

func addSyntheticCandidateDocs(tb testing.TB, root string, count int) {
	tb.Helper()
	for i := 0; i < count; i++ {
		rel := fmt.Sprintf("modules/generated-%04d.md", i)
		content := moduleFixture(
			fmt.Sprintf("合成模块 %04d", i),
			fmt.Sprintf("负责合成业务 %04d 的状态处理、校验和通知。", i),
			[]string{fmt.Sprintf("synthetic-%04d", i), "状态处理"},
			[]string{fmt.Sprintf("example/internal/generated.Service%04d.Handle", i)},
		)
		path := filepath.Join(root, ".repomind", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			tb.Fatalf("MkdirAll(%s): %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			tb.Fatalf("WriteFile(%s): %v", path, err)
		}
	}
}

func candidateRecallCases() []recallCase {
	return []recallCase{
		{name: "exact entitlement symbol", query: "RefreshEntitlementCache 执行后会员权益仍未到账", kind: KindTrouble, want: "troubles/vip-cache-delay.md"},
		{name: "exact migration symbol", query: "MigrateVipEntitlement 后老会员权益丢失", kind: KindTrouble, want: "troubles/vip-migration-missing.md"},
		{name: "exact payment symbol", query: "HandlePaymentCallback 返回成功但订单还是处理中", kind: KindTrouble, want: "troubles/payment-callback-pending.md"},
		{name: "exact chat symbol", query: "LoadSessionMessages 查不到历史聊天", kind: KindTrouble, want: "troubles/chat-history-empty.md"},
		{name: "business wording", query: "活动奖励显示成功但是钻石没有到账", kind: KindTrouble, want: "troubles/reward-grant-missing.md"},
		{name: "payment wording", query: "用户已经付款订单状态一直没有更新", kind: KindTrouble, want: "troubles/payment-callback-pending.md"},
		{name: "similar cache", similarTo: "troubles/vip-cache-delay.md", want: "troubles/vip-cache-stale-display.md"},
		{name: "similar migration", similarTo: "troubles/vip-migration-missing.md", want: "troubles/vip-migration-rollback.md"},
		{name: "similar payment", similarTo: "troubles/payment-callback-pending.md", want: "troubles/payment-callback-duplicate.md"},
		{name: "similar reward", similarTo: "troubles/reward-grant-missing.md", want: "troubles/reward-grant-idempotency.md"},
	}
}

func candidateFixture(tb testing.TB) string {
	tb.Helper()
	root := tb.TempDir()
	docs := map[string]string{
		"troubles/vip-cache-delay.md":            knowledgeFixture("VIP 权益延迟生效", "购买会员后权益未到账，首查权益缓存刷新与账务状态。", []string{"VIP", "会员权益", "未到账"}, []string{"example/internal/entitlement.(*Cache).RefreshEntitlementCache"}, "缓存刷新延迟造成页面仍显示旧权益。"),
		"troubles/vip-cache-stale-display.md":    knowledgeFixture("会员页面展示旧权益", "账务权益正确但会员页面仍展示旧值，检查权益缓存刷新链路。", []string{"VIP", "权益展示", "缓存"}, []string{"example/internal/entitlement.(*Cache).RefreshEntitlementCache"}, "缓存更新成功前不要判定账务发放失败。"),
		"troubles/vip-migration-missing.md":      knowledgeFixture("VIP 迁移后权益缺失", "账号迁移后老会员历史权益丢失，首查迁移批次与账户映射。", []string{"VIP", "会员权益", "账号迁移"}, []string{"example/internal/migration.MigrateVipEntitlement"}, "迁移遗漏旧账户映射时需要补偿。"),
		"troubles/vip-migration-rollback.md":     knowledgeFixture("会员迁移回滚不完整", "VIP 迁移任务回滚后部分权益未恢复，检查迁移映射和补偿记录。", []string{"VIP", "迁移回滚", "权益"}, []string{"example/internal/migration.MigrateVipEntitlement"}, "回滚必须覆盖已写入的新账户权益。"),
		"troubles/payment-callback-pending.md":   knowledgeFixture("支付成功订单仍处理中", "用户已经付款但订单状态一直未更新，首查支付回调消费与状态机。", []string{"支付", "订单处理中", "回调"}, []string{"example/internal/payment.(*CallbackService).HandlePaymentCallback"}, "回调落库后推进订单状态。"),
		"troubles/payment-callback-duplicate.md": knowledgeFixture("重复支付回调导致状态异常", "支付回调重复投递时订单状态推进异常，检查回调幂等记录。", []string{"支付", "重复回调", "订单状态"}, []string{"example/internal/payment.(*CallbackService).HandlePaymentCallback"}, "同一支付流水只能推进一次。"),
		"troubles/order-create-timeout.md":       knowledgeFixture("创建订单请求超时", "提交订单时接口超时，首查库存锁和下单事务，不检查支付回调。", []string{"订单", "超时", "库存"}, []string{"example/internal/order.(*Service).CreateOrder"}, "先区分请求超时与支付后状态未推进。"),
		"troubles/chat-history-empty.md":         knowledgeFixture("聊天历史查询为空", "进入已有会话后看不到历史消息，首查会话消息存储和租户条件。", []string{"聊天", "历史消息", "会话"}, []string{"example/internal/chat.(*Store).LoadSessionMessages"}, "确认会话标识和团队隔离条件。"),
		"troubles/reward-grant-missing.md":       knowledgeFixture("活动钻石发放未到账", "活动奖励显示成功但钻石没有到账，首查发奖事件消费。", []string{"活动奖励", "钻石", "发放"}, []string{"example/internal/reward.(*Service).GrantReward"}, "检查事件是否消费以及账本是否写入。"),
		"troubles/reward-grant-idempotency.md":   knowledgeFixture("奖励重试未补发", "活动发奖任务重试后仍无钻石，检查发奖幂等键和失败状态。", []string{"奖励重试", "钻石", "幂等"}, []string{"example/internal/reward.(*Service).GrantReward"}, "失败记录不能占用成功幂等状态。"),
		"modules/entitlement.md":                 moduleFixture("会员权益模块", "负责会员权益发放、缓存刷新与账号迁移后的权益恢复。", []string{"VIP", "权益", "会员"}, []string{"example/internal/entitlement.(*Cache).RefreshEntitlementCache"}),
		"concepts/vip.md":                        conceptFixture("VIP 会员", "付费会员身份与权益集合，用于判断购买、生效和续费边界。", []string{"VIP", "会员", "订阅"}),
	}
	for rel, content := range docs {
		path := filepath.Join(root, ".repomind", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			tb.Fatalf("MkdirAll(%s): %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			tb.Fatalf("WriteFile(%s): %v", path, err)
		}
	}
	return root
}

func knowledgeFixture(name, description string, keywords, codeRefs []string, detail string) string {
	return fixtureFrontmatter(name, description, keywords, codeRefs) + fmt.Sprintf("\n# %s\n\n## 问题现象\n\n%s\n\n## 排查方法\n\n%s\n\n## 结果判断\n\n按证据选择对应处理分支。\n", name, description, detail)
}

func moduleFixture(name, description string, keywords, codeRefs []string) string {
	return fixtureFrontmatter(name, description, keywords, codeRefs) + fmt.Sprintf("\n# %s\n\n## 模块职责\n\n%s\n\n## 包含能力\n\n- 权益处理。\n\n## 技术入口\n\n- 见 code_refs。\n", name, description)
}

func conceptFixture(name, description string, keywords []string) string {
	return fixtureFrontmatter(name, description, keywords, nil) + fmt.Sprintf("\n# %s\n\n## 这是什么\n\n%s\n\n## 核心规则\n\n- 以账务记录为准。\n", name, description)
}

func fixtureFrontmatter(name, description string, keywords, codeRefs []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %q\ndescription: %q\nstatus: active\n", name, description)
	if len(keywords) > 0 {
		b.WriteString("keywords:\n")
		for _, keyword := range keywords {
			fmt.Fprintf(&b, "- %q\n", keyword)
		}
	}
	if len(codeRefs) > 0 {
		b.WriteString("code_refs:\n")
		for _, codeRef := range codeRefs {
			fmt.Fprintf(&b, "- %q\n", codeRef)
		}
	}
	b.WriteString("---\n")
	return b.String()
}

func evaluateCandidateRecall(t *testing.T, root string, cases []recallCase) recallMetrics {
	t.Helper()
	ranks := make([]int, 0, len(cases))
	for _, tc := range cases {
		response, err := FindCandidates(root, CandidateOptions{Query: tc.query, SimilarTo: tc.similarTo, Kind: tc.kind, Limit: 10})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		ranks = append(ranks, candidateRank(response.Candidates, tc.want))
	}
	return metricsForRanks(ranks)
}

func evaluateLegacyRecall(t *testing.T, root string, cases []recallCase) recallMetrics {
	t.Helper()
	docs, err := scanDocuments(root)
	if err != nil {
		t.Fatal(err)
	}
	ranks := make([]int, 0, len(cases))
	for _, tc := range cases {
		query := tc.query
		kind := tc.kind
		seedFile := normalizeKnowledgePath(tc.similarTo)
		if seedFile != "" {
			for _, doc := range docs {
				if doc.File == seedFile {
					query = strings.Join(append([]string{doc.Name, doc.Description}, doc.Keywords...), " ")
					kind = doc.Kind
					break
				}
			}
		}
		type scored struct {
			file  string
			score int
		}
		var results []scored
		for _, doc := range docs {
			if doc.File == seedFile || kind != "" && doc.Kind != kind || doc.Status != "active" {
				continue
			}
			if score := legacyLexicalScore(doc, query); score > 0 {
				results = append(results, scored{doc.File, score})
			}
		}
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].score == results[j].score {
				return results[i].file < results[j].file
			}
			return results[i].score > results[j].score
		})
		rank := 0
		for i, result := range results {
			if result.file == tc.want {
				rank = i + 1
				break
			}
		}
		ranks = append(ranks, rank)
	}
	return metricsForRanks(ranks)
}

// legacyLexicalScore reproduces the deleted pre-ac23cdd field/body scorer.
// It intentionally has no code_refs or same-document similarity evidence.
func legacyLexicalScore(doc scannedDocument, query string) int {
	terms := legacySearchTerms(query)
	score := 0
	add := func(value string, weight, capMatches int) {
		hits := len(matchingTerms(value, terms))
		if hits > capMatches {
			hits = capMatches
		}
		score += hits * weight
	}
	add(doc.Name, 15, 4)
	add(doc.File, 4, 3)
	add(doc.Description, 7, 6)
	add(strings.Join(doc.Keywords, " "), 12, 4)
	var sectionScores []int
	for i, section := range doc.Sections {
		content := ""
		if i < len(doc.sectionContents) {
			content = doc.sectionContents[i]
		}
		value := len(matchingTerms(section.Title, terms))*5 + len(matchingTerms(content, terms))
		if value > 0 {
			sectionScores = append(sectionScores, value)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sectionScores)))
	for i, value := range sectionScores {
		if i >= 3 {
			break
		}
		if value > 12 {
			value = 12
		}
		score += value
	}
	normalized := strings.ToLower(cleanInline(query))
	all := strings.ToLower(doc.Name + "\n" + doc.Description + "\n" + strings.Join(doc.Keywords, "\n") + "\n" + doc.Body)
	if len([]rune(normalized)) >= 2 && strings.Contains(all, normalized) {
		score += 20
	}
	return score
}

func legacySearchTerms(query string) []string {
	query = strings.ToLower(cleanInline(query))
	segments := strings.FieldsFunc(query, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
	})
	stop := map[string]bool{
		"什么": true, "怎么": true, "如何": true, "是否": true, "这个": true,
		"一下": true, "问题": true, "系统": true, "功能": true,
	}
	seen := make(map[string]bool)
	var terms []string
	appendTerm := func(term string) {
		term = strings.TrimSpace(term)
		if term == "" || stop[term] || seen[term] {
			return
		}
		seen[term] = true
		terms = append(terms, term)
	}
	for _, segment := range segments {
		runes := []rune(segment)
		if containsHanRunes(runes) {
			if len(runes) <= 8 {
				appendTerm(segment)
			}
			for i := 0; i+1 < len(runes); i++ {
				appendTerm(string(runes[i : i+2]))
			}
			continue
		}
		if len(runes) >= 2 {
			appendTerm(segment)
		}
	}
	if len(terms) == 0 && query != "" {
		appendTerm(query)
	}
	return terms
}

func candidateRank(candidates []CandidateResult, want string) int {
	for i, candidate := range candidates {
		if candidate.File == want {
			return i + 1
		}
	}
	return 0
}

func metricsForRanks(ranks []int) recallMetrics {
	var top1, recall3, reciprocal float64
	for _, rank := range ranks {
		if rank == 1 {
			top1++
		}
		if rank > 0 && rank <= 3 {
			recall3++
		}
		if rank > 0 {
			reciprocal += 1 / float64(rank)
		}
	}
	n := float64(len(ranks))
	return recallMetrics{top1: top1 / n, recall3: recall3 / n, mrr: reciprocal / n}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
