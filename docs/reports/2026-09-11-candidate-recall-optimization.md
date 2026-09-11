# RepoMind 团队知识候选召回优化报告

日期：2026-09-11

## 结论

本次优化没有接入向量数据库，也没有新增 CLI 命令。现有 `kb-metadata` 增加了两个只读模式：

```bash
repomind kb-metadata --query "<用户问题>" --limit 5
repomind kb-metadata --similar-to "<知识文件>" --limit 5
```

候选排序综合稳定代码符号、名称、描述、关键词、文件名、章节标题和少量正文命中。排序只负责缩小候选范围，最终由 Agent 读取 Top-K 正文并执行 `SKIP / UPDATE / MERGE / CREATE`，不会因为关键词或符号相同就直接合并。

## 优化前的问题

优化前的 Skill 需要自行枚举和读取所有 Markdown frontmatter，再由模型比较 `name/description/keywords`。这有三个问题：

1. 不同 Agent 容易采用不同的命令组合和排序口径。
2. 相同业务词下存在多个入口时，纯词法排序难以区分实际代码对象。
3. Compact 虽然已经定义两阶段合并矩阵，但“如何找到同领域候选”没有可执行、可测试的实现。

仓库历史中的旧 `scoreDocument` 是最接近的确定性基线：它对名称、描述、关键词、文件和正文分节做词法加权，但不认识稳定代码符号，也没有 `similar-to` 的同文档证据。

## 实现设计

### 稳定代码锚点

知识 frontmatter 新增可选字段：

```yaml
code_refs:
- "example/internal/payment.(*CallbackService).HandlePaymentCallback"
```

Go 使用 `模块/package.(*Receiver).Method`；其他语言使用 `仓库相对文件#类或函数`。不保存容易随编辑漂移的行号。

涉及的主要代码符号：

- `kb.FindCandidates`：只读扫描、过滤、评分和 Top-K 排序。
- `kb.scoreCandidate`：生成分数、命中字段和可审计 reasons。
- `kb.splitFrontMatter` / `kb.renderDocument`：读写 `code_refs`。
- `cli.kbMetadataCmd`：复用现有命令暴露 query/similar 模式。

### 候选排序

排序优先级如下：

1. 完整 `code_ref` 命中或两个文档共享完整 `code_ref`。
2. 查询命中函数/方法叶子名称。
3. `name` 和 `description` 的业务语义词命中。
4. `keywords`、文件名和章节标题补充召回。
5. 正文只提供低权重兜底，避免长文天然占优。

`--similar-to` 默认只比较相同 kind，并排除种子文件自身。`draft` 和 `deprecated` 默认不参与，除非显式开启。
响应同时返回 `total_matches` 和 `returned`，Top-K 截断不会隐藏剩余候选数量。

### 合并裁决

三个 Skill 统一使用以下文档动作：

| 动作 | 判断标准 |
|---|---|
| `SKIP` | 旧文档已经完整覆盖，新内容没有增量或证据更弱 |
| `UPDATE` | 同一事实的新内容更新、更具体、更权威或纠正旧结论 |
| `MERGE` | 同一工作对象或诊断过程，信息互补且不冲突 |
| `CREATE` | Top-K 均不是同一对象，并能说明为何无法合入 |

候选命中不是合并决定。相同 `keywords` 只算弱证据；相同 `code_refs` 是强候选证据，但仍需比较症状、业务对象、首查入口和责任模块。

## 效果评测

### 质量口径

评测代码：`kb.TestCandidateRecallQualityAgainstLegacyLexicalBaseline`

- 语料：12 篇人工构造的 concept/module/trouble 文档。
- 标注：10 个查询或相似文档任务。
- 干扰设计：VIP、权益、支付、订单等词在多篇文档中重复，但入口函数和诊断目标不同。
- Baseline：复刻删除前的 RepoMind 词法评分逻辑，不使用 `code_refs`。
- Optimized：本次 `FindCandidates` 实现。

| 指标 | 优化前 | 优化后 | 变化 |
|---|---:|---:|---:|
| Top-1 Accuracy | 80.0% | 100.0% | +20.0 个百分点 |
| Recall@3 | 100.0% | 100.0% | 持平 |
| MRR | 0.900 | 1.000 | +0.100 |

这说明旧方案通常能把正确文档放进前三，但在同业务词的多个候选之间会排错第一名；稳定代码符号把正确候选提升到了首位。

### 性能口径

评测代码：`kb.BenchmarkCandidateRanking`

- 语料：512 篇 Markdown，其中 500 篇为干扰模块文档。
- 操作：每次从磁盘扫描、解析全部文档，完成评分和 Top-5 排序。
- 环境：Linux amd64，Intel Core Ultra 5 125H。
- 5 次结果：9.09 ms、9.13 ms、9.28 ms、9.19 ms、9.21 ms。

中位耗时约 **9.19 ms/次**，约 **3.55 MB/次**、**52,367 allocs/次**。当前规模下无需数据库或常驻索引；未来文档达到数千篇并出现性能瓶颈时，再增加按内容哈希更新的本地缓存即可。

## 安全与兼容性

- 没有新增第三方 Go 依赖。
- 没有 embedding API、密钥、网络、模型维度或向量重建问题。
- 无参数 `kb-metadata` 保持原有完整元数据输出行为。
- query/similar 模式直接调用只读 `scanDocuments`，不会触发迁移、Normalize 或生成文件写入。
- `scanDocuments` 现在递归覆盖 `concepts/modules/troubles` 子目录，并忽略符号链接，避免领域目录漏召回或链接越界读取。
- CLI 回归测试会比较查询前后知识文件内容，防止只读路径退化为隐式写入。
- 没有修改或清理用户已有的未提交产物、二进制及 graphify 输出。

## 局限与后续门槛

本次 100% 是受控合成标注集结果，不代表生产团队语料准确率。当前方案仍依赖文档拥有有区分度的 `description/keywords/code_refs`；完全不同的自然语言改写且没有共同符号时，可能漏召回。

建议收集真实使用中的 Top-K 结果和用户最终选择，累计至少 50-100 条匿名标注查询。只有出现稳定的低 Recall@3，且补充元数据仍不能解决时，才评估可选的章节级 embedding；届时仍可保留当前排序作为词法/符号通道，用 RRF 与向量结果融合。

## 验证命令

```bash
go test ./...
go vet ./...
go test ./internal/kb -run TestCandidateRecallQualityAgainstLegacyLexicalBaseline -v -count=1
go test ./internal/kb -bench BenchmarkCandidateRanking -benchmem -run '^$' -count 5
```
