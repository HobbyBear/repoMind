---
name: repomind-compact
description: 按用户指定的 RepoMind 文件、业务领域或整库范围做精简、领域聚类、诊断树合并和去重。适用于压缩 concepts、modules、troubles，尤其是把同一症状和首查入口下的多个 trouble 根因合成一篇可执行手册。局部模式严格限制正文读取和修改范围，领域模式先只读 frontmatter 发现候选并经用户确认扩围，整库模式执行全库一致性治理。始终只生成可审阅草案，不提交或推送。
metadata:
  short-description: 按指定范围精简 RepoMind 知识
---

# RepoMind 知识精简

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

目标是把 RepoMind 从 AI 工作日志整理成可执行的当前手册，并让同一业务问题的不同根因落在一篇分支清晰的诊断剧本中。用户指定几个文件或目录时，只处理这些正文；不要为了判断全局最优结构擅自扩大正文读取范围。

## 模式选择

开始前必须先确定模式：

- **局部模式**：用户指定了一个或多个文件、目录、glob 或明确说“只处理这些”。这是默认优先模式。
- **领域合并模式**：用户明确要求查找并合并指定 trouble 的同领域内容，或确认了局部模式给出的候选扩围清单。
- **整库模式**：用户没有指定范围，或明确要求全量/整库治理。

不得把局部模式静默升级为领域或整库模式。范围不足以确认跨文档冲突时，在结果中列为“范围外待确认”，不要读取范围外正文补证。

局部调用示例：

```text
$repomind-compact 合并并精简以下文件：
- .repomind/troubles/vip-not-effective.md
- .repomind/troubles/vip-migration-missing.md

$repomind-compact 只整理 .repomind/troubles/payment/ 目录
```

## 通用安全边界

- 只处理 `.repomind/concepts/**/*.md`、`.repomind/modules/**/*.md`、`.repomind/troubles/**/*.md`。
- `.repomind/README.md` 和 `.repomind/.generated/` 是可重建产物，不作为人工精简输入。
- 不修改业务代码，不执行 commit、push、checkout、reset、clean、stash。
- Git 历史保留演进过程；正文只保留当前有效事实。无法确认的新旧冲突写入最终报告，不猜测。
- 本流程产出审阅草案，发布权属于用户。

## 通用内容裁决

每条内容必须明确归入一种处理，不允许默认追加：

- `KEEP`：当前仍有效、代码不容易直接看出、对后续读者可复用。
- `REPLACE`：新结论修正旧结论；正文只留新结论，Git 保存旧版本。
- `MOVE`：事实放错了所有者；移到本次允许范围内的唯一主文档，其他位置只保留链接和一句上下文。
- `DISCARD`：过程日志、一次性 ID、原始报错堆栈、完整调查时间线、代码可直接读出的细节、重复表述。

唯一所有者规则：业务定义和规则归 `concepts`，模块职责、入口和改动风险归 `modules`，可复用的诊断剧本归 `troubles`。同一事实不得在多篇复制。

### Trouble 事件升维规则

以下压缩规则直接移植自 TencentDB 的 L1/L2 压缩与 Skill Review 提示词：

1. **任务对齐**：先识别原排查要解决的核心问题，只保留对该问题有实质影响的发现、动作、决策和阻塞。
2. **价值过滤**：忽略工具如何工作的冗余细节，提取“发现了什么关键线索、证实了什么假设、为什么走不通、最终改变了什么”。
3. **弹性聚合**：连续且意图相同的常规动作合并为一个宏观步骤；关键转折和重大发现才保留为独立判断分支，绝不记流水账。
4. **认知墓碑**：只有彻底走不通、容易重犯或会引发严重后果的方案才保留为“容易误判/禁止做法”；低价值失败直接删除。
5. **结论导向**：内容聚焦“得出了什么结论、靠什么证据、以后怎么做”，不罗列琐碎参数和调查经过。
6. **实事求是**：不把未发生的动作写成经验，不把猜测写成根因；每个结论必须能映射到用户确认、当前代码、工具结果或验证结果。

不得长期保留裸日志、一次性报错、自动恢复且无复用步骤的瞬态故障、真实业务 ID、精确日期、Owner、当前状态、完整调查时间线和修订记录。事故中的稳定业务规则移到 concept，模块入口和改动风险移到 module；`trouble` 保留为按症状召回的快速排查指南，无法提炼出指南的事故记录才删除。

候选召回优先使用 frontmatter `code_refs` 中稳定的函数、方法、类型或接口；不要把行号作为长期引用。相同 `code_refs` 是强候选证据，但最终仍需结合症状、业务对象和首查入口判断，不能自动合并。

压缩不能只以文件数或字节数下降为成功。表名、关键字段、状态码和稳定代码符号属于可执行证据锚点；合并时必须保留，或在来源映射中明确说明为何已失效。若调用方提供了独立的优化前项目副本，从优化后的项目根目录执行：

```bash
repomind kb-audit --compare-to "<优化前项目根目录或 .repomind>"
```

验收要求：`risks` 为空、已有 `code_refs` 保留率为 100%、证据锚点保留率不低于 80%，且严格校验通过。`quality_score` 只衡量结构质量；`lexical_unit_retention` 只衡量原句片段保留，语义改写会使它偏低。必须逐条复核 `potentially_lost_units` 和 `potentially_lost_anchors`，再用来源事实到目标分支的人工映射确认语义覆盖，不能为了提高字面分数复制旧文。

合并 trouble 时先判断是否属于同一诊断入口：

- 用户症状族、业务对象和公共首查入口相同：原则上合并为一篇诊断剧本；不同根因、数据状态和修复动作写成判断分支，不各建事故文档。
- 仅当首查入口、责任模块或核心证据系统确实不同，或合并后超过大小限制且无法形成清晰判断树时，才保留独立文档。
- 共享 VIP、支付、聊天等泛业务词不足以判定同类；`keywords` 也不是聚类依据。
- 对每个未合并的同领域候选，必须记录具体理由，不能只写“症状不同”或“证据不同”。
- `数据查询` 必须说明“查什么、得到什么信息、不同结果下一步做什么”，优先使用 `查询 | 能确认的信息 | 结果判断与下一步` 表格。
- trouble 不保留真实 UID、DID、订单号、日期案例、误判过程或修订时间线；这些由 Git 历史承担。

合并多个 trouble 时使用以下骨架；已有等价章节可以保留原名，没有内容的可选章节直接省略，不得制造空章节，也不得把原本嵌套的 `###` 无理由提升为 `##`：

```markdown
## 适用症状
## 首查步骤
## 判断分支
| 证据或条件 | 结论 | 下一步 |
```

只有确有跨案例价值时才增加 `修复原则`、`验证方式` 或 `容易误判`；没有内容就省略，不能为了模板完整而拉长指南。首屏必须能完成症状确认和第一次分流。

## Trouble 两阶段流程

只要范围内包含两个以上 trouble，或任务目标包含 trouble 去重，就必须分两阶段执行：

1. **聚类阶段，不编辑**：输出 `领域 -> 候选文件 -> 共同症状 -> 业务对象 -> 公共首查入口 -> 合并/保留理由` 的合并矩阵，选出拟保留主文档。
2. **整理阶段**：按已展示的矩阵合并判断分支，把来源事实逐条映射到主文档章节，删除已完全吸收的来源文件。

不能边读边改，也不能只做逐文件缩写。每个来源文件都必须有 `合并到 / 独立保留 / 丢弃` 三者之一的明确结论。

整理阶段对每篇候选明确执行一个文档动作：

- `SKIP`：主文档已完整覆盖，候选没有新增有效知识。
- `UPDATE`：候选给出了同一事实更具体、更新或更权威的结论，以新结论替换旧结论。
- `MERGE`：候选与主文档描述同一诊断过程且信息互补，融合为判断分支并删除已吸收来源。
- `CREATE`：没有同一工作对象的主文档，或入口和证据系统确实独立；必须写出无法并入 Top-K 候选的理由。

## 局部模式

### 1. 建立硬性范围

1. 将用户给出的文件和目录解析为仓库内真实路径；目录只递归展开通用安全边界允许的人工 Markdown。
2. glob 只能展开到允许目录内的人工 Markdown；拒绝越出 `.repomind` 的路径和符号链接逃逸。
3. 去重、排序并先向用户展示最终文件清单。该清单是本轮读取和修改的硬性 allowlist。
4. 如果没有匹配到文件，停止并报告，不回退到整库模式。

局部模式中：

- 只完整读取 allowlist 中的文件。
- 不读取 catalog、README、同目录其他文件、链接目标、源码或 graphify，除非它们本身在 allowlist 中。
- 不修改 allowlist 外的已有文件。
- 合并时默认从 allowlist 中选择一篇作为主文档；只有名称都不再准确时才可新建一篇同 kind 的目标文档，并在写入前把它加入输出清单。
- 可以删除已被合并的 allowlist 文件；不得删除范围外文件。
- 如果发现事实应归属到范围外文档，只在报告中提出 `MOVE` 建议，不执行移动。

### 2. 只建立局部基线

记录 allowlist 的文件数、总字节、行数、章节和当前校验问题。允许执行：

```bash
repomind kb-validate --file "<文件1>" --file "<文件2>"
```

不要执行 `kb-build`、`kb-audit`、无 `--file` 的 `kb-validate` 或其他整库读取。局部任务不验证范围外重复和冲突。

若要运行前后对比，优化前和优化后的临时项目都必须只包含 allowlist 文件；此时 `kb-audit --compare-to` 扫描的是隔离范围，不得用它读取原项目的范围外正文。

### 3. 在范围内精简与合并

- 先生成 `保留主文档 / 合并来源 / 删除来源 / 新建目标` 计划，再编辑。
- `description` 不超过 120 字，只说明典型症状或何时打开。
- `keywords` 只保留确有检索价值的词；删除泛词、长句和同义堆叠，不为凑数量新增关键词，也不以关键词决定合并关系。
- 删除修订记录、个案、调查过程和已被当前结论替代的说法。
- 删除“当前状态”“涉及模块”“实际案例”等事件档案章节；其中仍有效的状态分支改写为判断表，稳定入口移入 `code_refs` 或 module。
- 相同意图的短文合并；一篇混有多个诊断目标时，仅在 allowlist 内有合适目标时拆分。
- 不为了消除 warning 增加空洞章节、占位句或同义转述。

### 4. 只验收输出文件

对最终保留和新建的文件逐个执行：

```bash
repomind kb-validate --strict --file "<输出文件>"
```

不要把已删除文件传给校验命令。局部验收必须确认：

- 输出文件无 warning/error。
- 每份输入的有效事实都有明确去向，或记录了丢弃理由。
- trouble 文件数不得无理由增加；合并后的判断分支覆盖全部来源根因，标题层级没有被扁平化。
- 读取、修改、删除和新建清单都没有越出本次范围。
- 最终报告明确写出“未执行整库构建和审计；范围外一致性未验证”。

## 领域合并模式

领域模式只用于用户希望从指定 trouble 向外发现同类文档的场景：

1. 先建立用户指定文件的初始 allowlist。
2. 对每个初始文件执行 `repomind kb-metadata --similar-to "<文件>" --kind trouble --limit 10`。该模式只读扫描人工 Markdown，不运行迁移或构建；根据返回的 `total_matches/returned` 和 `code_refs/name/description/keywords/score/reasons` 列出候选及未展示数量，不自行组合 `find`、`rg`、`sed` 扫全库。关键词只能辅助召回，不能单独触发合并。
3. 展示 `初始文件 / 建议扩围文件 / 召回依据`，取得用户确认后，把确认项加入正文读取和修改 allowlist。未确认前停止，不读取其正文。
4. 对扩围后的 allowlist 执行 Trouble 两阶段流程和局部验收。不得读取或修改未确认候选的正文。

若用户在最初请求中已经明确授权“查找并合并这个领域的所有 trouble”，可以把候选清单作为执行前通知后继续，无需再次确认；仍须保留清单用于审计。

## 整库模式

### 1. 建立全库基线

依次执行 `repomind kb-build`、`repomind kb-audit`，记录总文档数、总字节、错误、警告和超长页。读取 `.generated/catalog.json`，再按业务簇分批读取所有人工 Markdown；不要把所有正文一次性塞入上下文。

### 2. 先聚类再整理

- concepts 按业务概念和边界聚类。
- modules 按职责、入口和影响面聚类。
- troubles 按“用户症状族 + 业务对象 + 公共首查入口”聚类，并执行 Trouble 两阶段流程。
- 每个簇先选唯一主文档并生成合并矩阵，再逐簇执行 `KEEP / REPLACE / MOVE / DISCARD`；不同根因优先成为同一文档的判断分支。

### 3. 修复入口与正文

- 每篇正文首屏回答“这是什么、什么时候看、最重要的结论”。
- 技术入口使用 `code_refs` 保存稳定函数/方法/类型标识，正文说明用途；不保存行号。
- 多条实现链路并存时标注 `current / conditional / legacy / deprecated` 和切换条件。
- 页面以 8 KiB 为建议上限，超过 12 KiB 必须缩减或按独立主题拆分。
- 单章节以 2 KiB 为建议上限，超过 4 KiB 必须拆分。
- 超过 150 行时移除个案、修订记录和长故障过程。
- 已作废但仍有查询价值的页面改为 `status: deprecated`；纯测试页、空页和无业务价值页面删除。

### 4. 整库验收

再次执行 `repomind kb-build`、`repomind kb-audit`、`repomind kb-validate`。错误数、超长页数和总字节数不得高于基线，`description_incomplete`、`generic_description`、`revision_log` 和 `placeholder_description` 必须为 0。

有优化前副本时，再执行 `repomind kb-audit --compare-to "<优化前项目根目录或 .repomind>"`，按通用压缩验收门槛复核证据和潜在丢失项。

整库验收还必须比较整理前后的 trouble 文件总数和每个领域的文档数：

- 同领域候选必须有合并结果或具体的不合并理由。
- 每个来源事实必须能追踪到目标文档章节或丢弃理由。
- trouble 文件数增加时，逐篇说明为何无法作为现有诊断树的分支；没有说明即验收失败。
- `kb-validate` 通过只代表格式合格，不能替代语义聚类、合并矩阵和标题层级检查。

## 最终输出

只输出：

- 执行模式和实际处理范围。
- 基线与结果指标。
- 涉及 trouble 时的合并矩阵，以及整理前后总数和各领域文档数。
- 保留、修改、合并、新建、删除和 deprecated 文件清单。
- 每个输入文件的有效知识去向或丢弃理由。
- 仍需人工确认的范围内冲突，以及局部模式下发现但未读取的范围外待确认项。

不要提交或推送。
