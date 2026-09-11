# RepoMind 压缩优化与 Chat 真实语料评测

日期：2026-09-11

## 结论

本次不只优化了搜索，还补上了可执行的压缩决策和前后对比门禁。真实语料测试将 5 篇同一诊断入口的年会员分期 trouble 合并为 1 篇诊断树：文档数减少 80.0%，字节数减少 61.0%，严格校验从 17 个 warning 降为 0，结构质量分从 78 升为 100，证据锚点保留 44/45（97.8%），自动风险项为空。

压缩后的字面单元保留率为 31.5%。这不是语义得分：判断表把多篇长句改写成了较短分支，字面指标会主动把改写列入复核清单。经来源到分支的人工核对，预先定义的 6 类根因分支和 7 个公共诊断步骤均被覆盖。

## 实测范围

语料来自 `/home/xch/goprojects/src/chat/apps/chat/.repomind`。为避免改动业务项目，只把下列 5 篇原文复制到隔离的临时项目中测试，源目录未被写入：

- `yearly-installment-clock-anchored-at-activation.md`
- `yearly-installment-clock-reset-by-reward-log-ctime.md`
- `yearly-installment-delayed-by-reward-log-ctime.md`
- `yearly-installment-double-first-and-misdirected.md`
- `yearly-installment-wrong-uid-silent-loss.md`

优化前副本：`/tmp/repomind-chat-compact-before.h2LbyZ`

优化后副本：`/tmp/repomind-chat-compact-after.1smGlJ`

优化后主文档：`.repomind/troubles/yearly-installment-diagnosis.md`

## 候选召回结果

以激活时钟问题为种子运行：

```bash
repomind kb-metadata \
  --similar-to troubles/yearly-installment-clock-anchored-at-activation.md \
  --kind trouble --limit 10
```

返回的 4 个候选正好是另外 4 篇同域文档，分数依次为 145、140、140、125。该步骤只找候选；是否合并仍按“共同症状、业务对象、公共首查入口”人工裁决，没有按关键词分数自动合并。

## 前后评分

| 指标 | 优化前 | 优化后 | 变化 |
|---|---:|---:|---:|
| 文档数 | 5 | 1 | -80.0% |
| 总字节 | 20,976 | 8,167 | -61.1% |
| 严格校验 warning | 17 | 0 | -17 |
| 超过 8 KiB 文件 | 0 | 0 | 不变 |
| 可比较结构质量分 | 78 | 100 | +22 |
| 重复单元比例 | 1.23% | 0 | -1.23 个百分点 |
| 稳定代码引用 | 0 | 5 | +5 |
| 证据锚点保留 | 45 | 44 | 97.8% |
| 字面单元保留 | 162 | 51 | 31.5% |
| 自动风险项 | - | 0 | 通过 |

现有 `kb-audit` 的整库分数是 95 -> 100；表中的 78 -> 100 来自新增的前后可比较口径。新口径满分 100，分别考察校验 25 分、去重 20 分、文件大小 15 分、路由元数据和代码锚点 15 分、必备章节 25 分。它衡量结构质量，不声称能自动证明语义等价。优化前 warning 包括 3 个过长 description、3 个修订记录和 11 个当前状态/案例/涉及模块等事件档案章节。

## 人工语义复核

| 来源主题 | 合并后分支 | 状态 |
|---|---|---|
| 激活门禁将整条分期时间轴后移 | 激活资格与订单时间分支 | conditional，已覆盖 |
| `reward_log.ctime` 重置 30 天间隔 | 日志时间覆盖旧进度分支 | current，已覆盖 |
| 最大期数与最大时间来自不同记录 | 期数/时间快照错配分支 | current，已覆盖 |
| 多 UID 回退导致发给旧账号 | 领取人选择与执行耦合分支 | current，已覆盖 |
| `LIMIT 1` 导致未激活账号静默跳过 | 多账号候选分支 | legacy，注明当前已缓解 |
| D0 首期按 UID 幂等导致双发 | 首期兼容分支 | legacy/data，注明当前订单日志检查 |

公共排查保留 7/7：订单、支付/试用状态、产品配置、DID 用户和激活资格、旧属性进度、订单级奖励日志、资产流水。另复核了当前代码的三个关键事实：

- `service.ScanYearlyInstallment` 当前遍历 DID 下全部 UID，但 `not_due` 或失败后仍可能继续检查旧 UID。
- `dal.(*subscriptionReward).GrantedProgress` 当前独立聚合最高期数和最晚时间，两者可能不属于同一行。
- `service.SendSubscriptionFirstReward` 当前同时检查旧 UID 标记和订单级首期日志；历史缺日志数据仍需兼容。

字面复核清单促使合并稿补回 `user_attributes_info`、`gp_cancel_reason`、`GrantedIndex`、`GrantedAt` 等诊断证据，以及“无服务端历史日志时只能标高概率”的证据强度限定。唯一未保留锚点是 `vip_reward` 文件名；它被 5 个稳定函数/方法 `code_refs` 替代，符合不使用易漂移文件位置作为主引用的目标。

## 实现改动

1. 扩展现有 `kb-audit`：在优化后项目中运行 `repomind kb-audit --compare-to <优化前根目录或 .repomind>`，输出前后快照、缩减比例、重复率、字面保留、证据锚点、代码引用、潜在丢失项和风险项。
2. 证据锚点提取聚焦表名、字段名、状态码和代码标识；比较时把 `LastPaidAt` 与 `last_paid_at` 等命名风格视为同一标识，减少格式误报。
3. 压缩 Skill 增加硬门槛：风险项为空、原有 `code_refs` 100% 保留、证据锚点至少 80%、严格校验通过，并要求逐条人工复核潜在丢失项。
4. 同类文档继续使用 `SKIP / UPDATE / MERGE / CREATE` 裁决和两阶段合并矩阵，压缩不再只是逐文件缩写。
5. 添加知识层、CLI 层和 Skill 安装层回归测试。
6. 直接移植 TencentDB 团队工作记忆、Skill Review 和 L1/L2 压缩提示词中的价值过滤、独立完整、准确归因、弹性聚合、认知墓碑与低价值丢弃规则；事故只作为提炼输入，`trouble` 只保存可复用诊断方法。
7. `kb-validate` 新增 `trouble_event_section` warning。对 Chat 全库只读审计时，111 篇 trouble 命中 220 个“当前状态、涉及模块、实际案例”等事件档案章节，可作为后续整库压缩清单。

## 命令与依赖

运行时没有新增 CLI 命令，也没有接入向量数据库或外部命令行工具。Skill 使用 RepoMind 自己的三个入口：

```bash
repomind kb-metadata --similar-to <文件> --kind trouble --limit 10
repomind kb-validate --strict --file <输出文件>
repomind kb-audit --compare-to <优化前项目>
```

对比算法只使用 Go 标准库和现有 Markdown/frontmatter 解析结果。开发阶段使用 `go test`、`go vet` 和 `go build` 做工程验证，这些不是 Skill 的运行依赖。

## 局限

- 字面保留只能发现需要复核的改写，不能判断两个不同句子是否语义等价。
- 证据锚点能阻止关键标识无声丢失，但不能验证业务结论本身是否正确。
- 原始 5 篇没有 `code_refs`，因此本次只能报告新增 5 个稳定引用，不能用该样本验证“旧引用保留率”；对应丢失测试已由自动化用例覆盖。
- 本次只验证一个真实 trouble 簇，不代表对 186 篇文档完成了整库压缩。
