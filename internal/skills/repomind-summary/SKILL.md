---
name: repomind-summary
description: 每次 AI 完成代码修改、问答或排查结论后，且最终答复前，必须同步做轻量 summary gate 判断；代码写完后也要自动进入 gate。只有发现可复用的新业务知识、用户纠错、手动要求记忆的经验、模块边界、排查经验或知识文档元数据变化时，才执行写入。按类型更新 concepts、modules、troubles 及每个文档自己的 name/description，避免回到集中式索引。
metadata:
  short-description: 把可复用知识写回 RepoMind
---

# RepoMind 编码 / 问答 / 排查后更新

## CLI 环境预检

执行本 Skill 的任何其他步骤前，先检查 RepoMind CLI；已存在时只验证，不下载或更新。

macOS / Linux：

```bash
if ! command -v repomind >/dev/null 2>&1; then
  curl -fsSL https://raw.githubusercontent.com/HobbyBear/repoMind/master/install.sh | bash
  export PATH="/usr/local/bin:$HOME/.local/bin:$PATH"
  hash -r 2>/dev/null || true
fi
REPOMIND_BIN="$(command -v repomind)"; "$REPOMIND_BIN" --help >/dev/null
```

Windows PowerShell：

```powershell
if (-not (Get-Command repomind -ErrorAction SilentlyContinue)) {
  iwr -useb https://raw.githubusercontent.com/HobbyBear/repoMind/master/install.ps1 | iex
  $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")
}
$repomindBin = (Get-Command repomind -ErrorAction SilentlyContinue).Source; if (-not $repomindBin) { throw "RepoMind CLI 安装后仍不在 PATH" }
& $repomindBin --help | Out-Null
```

只使用上述 RepoMind 官方安装地址。不得只修改 shell 配置或系统环境变量后等待新终端：当前进程必须立即刷新 PATH 并解析出绝对路径。后续代码块中的 `repomind` 应使用已解析的 `$REPOMIND_BIN` / `$repomindBin` 执行；若新的工具调用启动了独立 shell，先重复 PATH 刷新和路径解析。下载、安装、刷新或 `--help` 验证任一步失败时立即停止，不得继续读写知识库或静默改用源码构建。

## 所有权与调用边界

- 本 skill 由 RepoMind 维护并随 `repomind install/update` 部署。
- FixForge 等外部系统只负责同步触发本 skill，不复制总结规则。
- Markdown 正文和 frontmatter 是人工可审阅的数据源；本 skill 直接维护它们。

**执行语义：本 skill 必须同步完成。**

- 调用方不能把它当后台任务，也不能在它完成前先回复用户
- AI 完成代码修改、生成文件、修复 bug 或跑完验证后，最终答复前也必须进入本 skill 的 summary gate
- 如果调用方只是口头说“summary 正在运行”但没有真正执行到清理和摘要输出，这视为流程失败
- 本 skill 完成的标志是：知识库更新/判定结束 + 清理 `.query-findings.json` + 输出 summary 摘要

## 核心原则

1. 先做 summary gate，只有值得沉淀才写文件。
2. RepoMind 不再维护 `index.json`；路由元数据写在各知识文档自己的 frontmatter。
3. 每次进入 summary，都要先用 `kb-metadata --query` 召回已有候选；稳定 `code_refs` 优先于会漂移的文件行号，`description` 和 `keywords` 补充业务语义。
4. 只记录“代码不会直接告诉你的东西”。
5. 用户纠正业务事实、模块归属、入口位置、排查根因或历史结论时，视为用户确认的修订证据，必须进入完整 summary 流程。
6. 用户明确要求“记一下 / 总结到知识库 / 以后遇到这个要注意 / 这个经验要沉淀”时，视为手动沉淀请求，必须进入完整 summary 流程。
7. 知识正文是“当前手册”，不是对话或修改日志。每条发现先判断 `KEEP / REPLACE / MOVE / DISCARD`，禁止默认追加到文末。
8. 同一事实只有一个主文档：业务规则归 concept，代码入口和改动风险归 module，排查路径归 trouble；其他文档只保留链接和必要上下文。
9. 新结论推翻旧结论时直接替换当前正文。完整演进由 Git 历史承担，不在正文累计时间线和修订流水。
10. 已作废但仍有历史查询价值的文档使用 `status: deprecated`，默认路由不再命中；测试页、空页和无业务价值内容直接删除。
11. 同一规则出现冲突时先停下追加，定位唯一权威定义和当前证据。证据不足则记录待确认冲突，不能同时把两种说法写成“当前规则”。
12. 事故、工单和排查会话只是提炼输入，不保存事故档案；`trouble` 类型保留，只作为按症状召回、未来可直接执行的快速排查指南。

## 人工内容保护

- 只修改本次知识涉及的章节和 frontmatter 字段，不整体重写文档。
- 人工正文与 AI 实际读取的正文必须一致，不允许在隐藏生成文件里另写一套业务结论。
- 发现同一章节在本轮已被人工修改时，保留人工内容，使用增量合并并明确冲突点。
- `name/description/keywords/code_refs` 是人工可见的检索字段；`code_refs` 只写稳定函数、方法、类型或接口锚点，不写行号。
- 产品、运营未完成的内容使用 `status: draft`；本 skill 只有在内容完成且严格校验通过后才改为 `status: active`。
- “只修改相关章节”不等于只追加：相关章节已有过时或重复内容时，必须就地替换、合并或删除。
- 页面已超过建议大小时，本轮只允许保持或降低其字节数与重复度；不得借更新继续扩张历史债务。

## 步骤 1：执行 summary gate

先回答四个问题：

| 问题 | 判断 |
|------|------|
| 是否有新知识 | ✅/❌ |
| 是否可复用 | ✅/❌ |
| 是否有证据 | 用户确认 / 当前代码 / 排查结果 / 现有知识库 |
| 推荐写入目标 | concepts / modules / troubles / discard |

gate 不通过时，直接输出“无需更新”，不要写文件。

### Trouble 二次准入门槛

以下规则直接移植自 TencentDB 团队工作记忆与 Skill Review 提示词。通用 gate 通过不代表必须写 `trouble`；候选排查经验还必须单独通过本门槛：

1. **面向工作协作**：提取出的记忆应能帮助团队成员或 Agent 在后续任务中理解项目背景、复用经验或避免重复错误。
2. **独立完整**：每条记忆必须跳出当前对话仍能理解，包含清晰的工作对象、触发症状、判断证据和可执行动作。
3. **准确归因**：建议、担忧和假设不等于已确认根因；只有用户确认、当前代码、工具结果或验证结果才能写成确定结论，其他内容必须标明证据强度。
4. **归纳合并**：强关联或有因果关系的多条消息必须合并为一条完整经验，不把同一事故拆成多个碎片。
5. **复用价值**：内部按 0-100 判断；90-100 是长期稳定、可跨任务复用的核心方法，70-89 是对项目后续明显有用的方法，低于 70 直接 `DISCARD`，分数不写入正文。

以下内容不得创建或扩充 `trouble`：

- 裸日志、原始报错或没有诊断路径的一次性错误。
- 自动恢复且没有可复用步骤的瞬态故障。
- 单次事故的 UID、DID、订单号、请求 ID、精确日期、Owner、deadline 和当前处置状态。
- 完整排查时间线、修订记录、未被采纳的 AI 建议、临时草稿和代码可直接看出的事实。

事件本身被丢弃，不等于丢弃经验：先尝试提取“以后遇到什么症状，按什么证据判断，应该怎么做、不要怎么做”。提取不出来才 `DISCARD`。

这里的“新知识”不只包括业务规则，还包括：

- 现有模块文档没有覆盖到的关键入口
- 现有模块关键词没有覆盖到的常见搜索词/别称
- 现有模块文档没有写出的常见修改场景

也就是说：**只要本轮代码查找暴露出 RepoMind 路由缺口，这本身就是需要 summary 的新知识。**

但是以下场景默认 **gate 通过，必须进入完整 summary 流程**：

- 本轮有业务代码修改
- 本轮有业务/排查/PRD 相关结论
- 本轮用户纠正了业务事实、模块归属、入口位置、排查根因或历史结论，例如“X 才是”“Y 错了”“不是 A，是 B”
- 本轮用户明确要求沉淀知识，例如“记一下”“总结到知识库”“以后遇到这个要注意”“这个经验要沉淀”
- 本轮为了定位代码，绕过了现有 `modules` 文档，转而直接查 graphify/source/`rg`
- 本轮识别出应该新增、删除或收紧某个模块的 `keywords`

## 步骤 2：读取待处理发现

优先读取：

```bash
cat .repomind/.query-findings.json 2>/dev/null || echo '{"needs_summary": false}'
```

如果没有这个文件，但本轮问答/排查确实形成了新知识，就按同样格式自行生成一个临时发现文件再继续。

如果没有这个文件，但用户在本轮明确纠正了旧说法，就按同样格式自行生成临时发现文件，并至少记录：

- 旧说法
- 新说法
- 证据来源（用户确认 / 当前代码 / 排查结果 / 现有知识库）
- 影响范围（concept / module / trouble）

如果没有这个文件，但用户明确要求“记一下 / 总结到知识库 / 以后遇到这个要注意 / 这个经验要沉淀”，就按同样格式自行生成临时发现文件，并至少记录：

- 用户要求沉淀的原始要点
- 这条知识的复用场景
- 证据来源（用户确认 / 当前代码 / 排查结果 / 现有知识库）
- 推荐写入目标（concept / module / trouble）

如果本轮存在“直接查代码才完成定位”的情况，即使 `.query-findings.json` 还没写，也必须自行补一份临时发现文件，至少包含一条 `module_knowledge`。

兼容旧类型时，先做归一化：

- `new_business_card` / `new_business_rule` → `concept_knowledge`
- `module_update` / `new_code_location` / `index_knowledge` → `module_knowledge`
- `trouble_record` → `trouble_knowledge`
- 无法归入具体 concept/module/trouble 的宽泛项目描述 → `discard`

## 步骤 3：先读取元数据，再定位要改的文档

用本轮发现的原始问题、业务对象、症状和代码符号执行：

```bash
repomind kb-metadata --query "<发现的业务语义和代码符号>" --limit 5
```

如果发现来自一篇已知知识文档，还要执行 `repomind kb-metadata --similar-to "<文件>" --limit 5`。这两个模式只读，不执行构建或迁移。根据返回的 `code_refs/name/description/keywords/score/reasons` 选择要打开的 1-3 篇正文。

不要直接全量打开所有 `concepts/*.md`、`modules/*.md`、`troubles/*.md`。

候选召回只负责缩小范围，不直接决定合并。打开候选正文后，对每条待沉淀知识选择一个文档动作：

| 动作 | 条件 |
|------|------|
| `SKIP` | 旧文档已经完整覆盖，新信息无增量或证据更弱 |
| `UPDATE` | 同一事实的新结论更具体、更新、更权威或纠正旧结论 |
| `MERGE` | 同一工作对象或诊断过程，信息互补且不冲突 |
| `CREATE` | Top-K 候选均不是同一对象，并能给出无法合入的具体理由 |

默认倾向 `UPDATE/MERGE`，但相同 `keywords` 或同属一个大模块不构成合并依据。发生冲突且证据不足时不得自动覆盖。

进入正文合并前，先单独判断：

- 这次发现是否改变了文档的适用场景、业务边界、典型现象或常见叫法
- 如果改变了，即使正文只改一点点，也要优先刷新 frontmatter 元数据

## 知识写入边界

### concepts

写：

- 业务定义
- 存在目的
- 用户侧表现
- 数据流
- 核心规则和边界
- 易混淆概念

不写：

- SQL、字段、函数步骤、调用链

frontmatter `description` 必须覆盖：

- 这个概念是什么
- 它会在哪些场景出现
- 它和什么最容易混淆，或主要边界是什么

### modules

写：

- 模块职责变化
- 关键入口
- 常见修改场景
- AI 注意事项
- 跨模块约束和隐性依赖

不写：

- 函数内部伪代码
- 长调用链
- 代码里 30 秒内能直接读到的普通事实

frontmatter `description` 必须覆盖：

- 模块负责什么业务
- 何时应该打开
- 典型影响面或风险

frontmatter `code_refs` 必须覆盖真正稳定的入口：

- Go 使用模块/package/receiver/方法，例如 `example/internal/payment.(*Service).HandleCallback`
- 其他语言使用 `仓库相对文件#类或函数`
- 同一符号只保留一次，不保存行号、commit hash 或大段调用链

所有知识文档都可以用 `keywords` 记录用户实际叫法；modules 必须维护。关键词要求：

- 模块名、常见别称、英文名或缩写
- 最常拿来搜它的业务词或入口词
- 只放 3-8 个判别词，不堆泛词

## 压缩与拆分门槛

以 `repomind kb-validate` 输出为准：

- 关键词超过 8 个：删除泛词、同义重复词和只能命中当前一次对话的长句。
- 单个关键词超过 32 个字符：改成用户真正会搜索的短语。
- 文件超过 12 KiB 或章节超过 4 KiB：生成拆分方案，按独立业务主题拆分。
- 文件超过 24 KiB：属于阻断错误，完成拆分前不得清理 `.query-findings.json`。
- 拆分后每篇文档必须拥有独立 `description/keywords`，原文保留“关联知识”链接，避免旧入口失效。

### troubles

定位：`trouble` 是查询问题时的快速指南，不是事故故事或复盘文档。打开后首屏应直接回答“是否适用、先查什么、不同证据分别怎么处理”。

写：

- 可复用的症状族和适用边界
- 公共首查步骤
- `证据或条件 -> 结论 -> 下一步` 判断分支
- 修复原则、验证方式、禁忌和容易误判点

不写：

- 某次事故发生、处理和结束的叙事
- 原始日志、堆栈、长 SQL 输出和真实业务 ID
- 当前状态、涉及模块清单、Owner、版本和修订流水
- 只有根因结论但没有识别证据或下一步的“答案卡”

frontmatter `description` 必须覆盖：

- 典型症状
- 首查方向 / 常见根因范围

## 步骤 4：分拣发现类型

把发现分成三类：

| 类型 | 写入目标 |
|------|----------|
| `concept_knowledge` | `.repomind/concepts/*.md` |
| `module_knowledge` | `.repomind/modules/*.md` |
| `trouble_knowledge` | `.repomind/troubles/*.md` |

如果只是一次性上下文、单次事故状态、纯代码显式信息或证据不足，归为 `discard`。事故中提取出的稳定业务规则归 concept，代码入口和改动风险归 module，只有可复用诊断方法归 trouble。

在分拣完成后，先做一次“元数据总结”：

- 哪些文档的 `description` 应该重写或收紧
- 哪些模块文档的 `keywords` 应该新增、删除或去重
- 即使正文改动很小，只要索引入口词变了，也必须优先更新元数据
- 如果本轮代码定位绕过了现有模块文档，也必须把“为什么没命中”“缺了什么关键词/入口词”总结到这里
- 如果本轮是用户纠错，必须判断被修正的是概念边界、模块归属、关键入口还是排查根因，并用新说法替换对应正文；旧说法交给 Git 历史
- 如果本轮是手动沉淀请求，必须判断它更像业务概念、模块修改经验还是排查经验；只写入 concepts/modules/troubles，不直接修改生成目录

## 步骤 5：更新知识文档

### 5a：concepts

模板：

```markdown
---
name: "Pro 角色"
description: "高级用户身份概念。用于判断权益范围、典型触发场景，以及和 VIP 的区别。"
status: active
---

# 概念：Pro 角色

## 这是什么

（一句话业务定义，回答“这个概念本身是什么”）

## 核心规则

（稳定规则、边界条件、负向规则；按主题合并，不抄实现分支）

## 适用场景与边界

（用户在哪些流程会遇到、它不是什么、和相邻概念有什么区别）

## 关联知识

（链接相关 concept/module/trouble；没有可暂时留空）
```

规则：

- 无卡片 + 有稳定业务语义 → 新建
- 有卡片 + 新增业务规则/边界/预期 → 合并
- 只是实现调整 → 不更新正文，但说明原因
- 用户纠正业务定义、业务规则、边界或易混淆概念 → 合并为当前有效结论，并保留必要的修订说明
- 如果卡片更新后适用场景或边界变化，必须同步改 `description`
- 每次 summary 都要问一句：当前 `description` 是否仍能让模型在首轮路由时命中这张卡；如果不能，先改 `description`

### 5b：modules

模板：

```markdown
---
name: "支付模块"
description: "支付与退款相关模块。用于定位下单、回调、补偿入口和改动影响面。"
status: active
keywords:
- "支付"
- "payment"
- "退款"
- "refund"
- "回调"
code_refs:
- "example/internal/payment.(*CallbackService).HandlePaymentCallback"
---

# 支付模块

## 模块职责

（1-3 句话描述模块在业务上负责什么，不要变成目录树说明）

## 包含能力

（列模块对外提供的主要业务能力）

## 技术入口

（列真正该看的入口文件/函数，并说明为什么从这里开始）

## 关键约束

（写隐性约束、跨模块联动、业务坑点和容易误改的地方）

## 关联知识

（链接相关 concept/module/trouble）
```

规则：

- 只保留有复用价值的模块知识
- 优先维护 `关键约束`
- 技术入口优先维护到稳定函数、方法、类型或接口，并同步写入 `code_refs`；正文可补充文件路径和入口用途
- 如果模块职责、入口范围、影响面发生变化，frontmatter `description` 必须同步更新
- 如果模块新增别称、核心入口词、常见搜索词或业务叫法变化，frontmatter `keywords` 必须同步更新
- 用户纠正模块归属、关键入口或常见叫法时，必须同步更新 `技术入口`、`包含能力` 或 `keywords`
- `description` 写不下的检索词，不要硬塞进句子，放进 `keywords`
- `keywords` 以命中率为目标，不以完整性为目标；去掉噪音词，保留最能区分该模块的词
- 如果本轮是靠直接源码/图谱搜索才找到该模块实现，必须反向补齐模块文档：
  - 补入口文件/函数
  - 补包含能力或常见修改场景
  - 补能帮助首轮命中的 `keywords`
  - 如有必要，收紧 `description`

### 5c：troubles

模板：

```markdown
---
name: "VIP 延迟生效"
description: "处理 VIP 购买后权益未及时生效时查看。包含首查方向和常见根因。"
status: active
code_refs:
- "example/internal/entitlement.(*Cache).RefreshEntitlementCache"
---

# VIP 延迟生效诊断

## 适用症状

（描述可重复出现的症状族和不适用边界，不写某次事故）

## 首查步骤

（只保留所有根因共享的 2-5 个检查，写清查什么）

## 判断分支

| 证据或条件 | 结论 | 下一步 |
|---|---|---|
| （可复现的证据组合） | （已确认根因或待确认假设） | （继续检查、修复或停止条件） |

## 修复原则

（保留跨案例成立的修复约束和禁止做法，不记录当次修改过程）

## 验证方式

（写能证明问题已解决且没有引入回归的检查）

## 容易误判

（只写高价值反模式：错误判断、为什么错、应看什么证据）

## 关联知识

（链接相关 module/concept/trouble）
```

规则：

- `SKIP`：已有记录完整覆盖且更清晰
- `MERGE`：同一症状、业务对象和公共首查入口下补充了根因或判断分支
- `UPDATE`：旧结论已过时，直接改成当前有效结论；历史交给 Git，不在正文追加修订时间线
- `CREATE`：通过 Trouble 二次准入门槛，Top-K 中没有同一诊断入口，并明确记录不能合并的原因；一次新事故本身不是创建理由
- 如果问题的典型症状或首查方向发生变化，frontmatter `description` 也要更新
- `修复原则`、`验证方式`、`容易误判` 都是可选章节，没有跨案例价值时直接省略；不得为了套模板拉长快速指南

## 步骤 6：严格校验本次写入

写完后执行：

```bash
repomind kb-validate --strict --file "<本次写入文件>"
```

每个新建或实质更新的知识文件都必须单独通过 strict 校验，确认 frontmatter、必备章节、关键词和体积限制无 warning/error。通过后才执行：

```bash
rm -f .repomind/.query-findings.json
```

## 步骤 7：输出摘要

摘要必须包含：

- Summary gate 结果
- 哪些发现被写入，哪些被丢弃
- concept/module/trouble 各自的更新动作
- 哪些文档的 frontmatter 元数据被同步调整
- 哪些模块关键词被新增、删除或去重
- 如果本轮有绕过模块文档的直接代码查找，必须写明：
  - 绕过了哪个模块或哪个缺口
  - 本次补回了哪些入口信息
  - 本次补回了哪些关键词

建议格式：

```markdown
## RepoMind 更新摘要

### Summary Gate
- 是否有新知识：...
- 是否可复用：...
- 写入目标：...

### 知识写入路由
- 概念 A → concepts/xxx.md
- 模块 B → modules/xxx.md
- 排查 C → troubles/xxx.md
- 丢弃 D → 原因

### 元数据同步
- concepts/xxx.md：更新 description，补充适用场景
- modules/yyy.md：更新 description，补充影响面
- modules/yyy.md：更新 keywords，补充新的搜索词 / 别称 / 入口词
```
