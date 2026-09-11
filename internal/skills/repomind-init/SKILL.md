---
name: repomind-init
description: 初始化 RepoMind 业务知识库。全量重建 graphify 图谱，保守生成高置信 concepts/modules 文档，并为每个知识文件写入 name/description 元数据。执行前会自动修正旧格式；初始化结束后会询问是否继续导入 PRD 业务知识。
metadata:
  short-description: 初始化 RepoMind 知识库
---

# RepoMind 初始化知识库

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

当 `.repomind/modules/` 下还没有有效模块文档，或现有知识库明显为空时执行本流程。

## 核心原则

1. graphify 图谱必须全量重建，不能偷用旧结果。
2. 知识库写入必须合并，不能覆盖已有人工沉淀。
3. RepoMind 不再维护集中式 `index.json` 或目录 README 作为路由入口。
4. 路由依赖每个知识文件自己的 frontmatter 元数据；`description` 提供业务语义，`code_refs` 提供稳定代码入口，`keywords` 补充常见叫法。

```yaml
---
name: "..."
description: "..."
code_refs: []
---
```

## 元数据约束

所有 `concepts/*.md`、`modules/*.md`、`troubles/*.md` 都必须满足：

- `name`：当前文件的规范名称，供大模型做首轮匹配。
- `description`：只写 1-2 句话，专门用于“是否该打开这份文档”的判断；这是首要索引摘要，优先级高于正文润色。
- `code_refs`：可选的稳定函数、方法、类型或接口锚点；Go 使用模块/package/receiver/方法，其他语言使用 `仓库相对文件#类或函数`，不要写行号。

`concepts` 的 `description` 必须包含：
- 这个概念是什么。
- 它会在哪些业务场景/产品语境中被提及。
- 它和哪个相邻概念最容易混淆，或核心边界是什么。
- 不要写代码路径、函数名、SQL、字段名。

`modules` 的 `description` 必须包含：
- 模块承担的业务职责。
- 什么时候应该打开这份文档。
- 典型影响面或跨模块风险是什么。
- 不要把整份文件清单塞进描述。

`modules` 还必须维护 `keywords`：
- 放 3-8 个真正能帮助路由的判别词。
- 优先放：模块名、英文名、核心业务词、核心入口词、常见别称。
- 不要放泛词，例如“系统”“功能”“业务”“模块”。
- 如果模块改动后，用户可能用新的叫法来找它，必须补到 `keywords`。

`troubles` 的 `description` 必须包含：
- 典型现象或触发条件。
- 首查方向、常见根因范围或受影响面。
- 让模型一眼判断“这是不是我要排查的问题”。

## 步骤 0：先构建当前知识库

在读取或写入任何知识文件前，先执行：

```bash
repomind kb-build
```

这一步必须每次执行。它会：

- 给旧文档补齐 frontmatter 的 `name` / `description`
- 把历史 `index.json` 中还能复用的信息迁入模块文档
- 删除过时的集中式索引文件和目录 README
- 为未来的格式演进保留长期迁移入口
- 生成 `.repomind/README.md` 人类导航页和 `.generated/catalog.json` 机器目录

从这一步开始，只允许按新格式继续工作，不要再回写旧结构。

## 步骤 1：全量重建图谱

调用 graphify skill 做全量分析，不是增量更新：

- Claude Code：`/graphify .`
- Codex：`$graphify .`

完成后继续，不要停在图谱阶段。

## 步骤 2：确保关键文件可提交

确认以下文件会被 git 跟踪：

- `graphify-out/graph.json`
- `graphify-out/GRAPH_REPORT.md`
- `graphify-out/manifest.json`
- `graphify-out/graph.html`
- `graphify-out/.vocab.txt`
- `.repomind/concepts/**`
- `.repomind/modules/**`
- `.repomind/troubles/**`
- `.repomind/.kb-format.json`
- `.repomind/README.md`

如果需要，执行：

```bash
git add graphify-out/ .repomind/
```

## 步骤 3：运行 graph-scan

```bash
repomind graph-scan
```

它会生成 `.repomind/graph/summary.json`，用于辅助判断：

- `module_candidates`
- `entry_files`
- `communities`
- `symbols`

## 步骤 4：先读元数据，再决定打开哪些文档

先读取知识库元数据，而不是盲扫全文：

```bash
repomind kb-build
repomind kb-validate
repomind kb-metadata
```

如果当前知识库为空，继续创建；如果已有文档，先看元数据决定哪些旧文档需要合并。

## 步骤 5：归纳业务模块和业务概念

基于目录结构、graph summary、入口文件和命名语义，先做高置信筛选。

候选模块分三类：

| 类型 | 处理方式 |
|------|----------|
| 业务模块 | 创建或合并 `.repomind/modules/*.md` |
| 技术支撑模块 | 只有承载业务入口或跨模块业务约束时才建模块文档 |
| 忽略目录 | 不建模块文档 |

候选概念分三类：

| 置信度 | 标准 | 处理 |
|--------|------|------|
| 高 | 在接口、服务、模型、配置、用户侧表现中反复出现 | 创建/合并 concept 卡片 |
| 中 | 名称像业务能力，但证据不足 | 只写入初始化摘要的待确认项 |
| 低 | 技术名词、字段名、内部工具名 | 丢弃 |

初始化只生成高置信知识，不要为了凑数量造概念。

## 步骤 6：创建或合并知识文档

### 6a：concepts

对每个高置信概念：

1. 如果已存在对应文档，先读取原文并合并。
2. 如果不存在，新建为：

```markdown
---
name: "Pro 角色"
description: "高级用户身份概念。用于判断权益范围、典型触发场景，以及和 VIP 的区别。"
status: active
---

# Pro 角色

## 这是什么

（一句话定义这个概念本身。回答“它到底是什么业务对象/能力/身份”，不要写实现）

## 核心规则

（写稳定业务规则、边界条件、负向规则。适合按小主题分组，不要抄 if/else）

## 适用场景与边界

（写“它不是什么”“和谁容易混”“区别是什么”，帮助后续问答时避免混淆）

## 关联知识

（链接相关 concept/module/trouble）
```

写 concept 时：

- 优先写业务定义、用户感知、数据流、边界。
- 不要把实现细节抄成业务卡片。
- `description` 必须能帮助后续“概念型问题”命中这张卡。

### 6b：modules

只对业务模块创建或合并模块文档。模板：

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

（1-3 句话说明这个模块承担的业务职责，回答“这个模块在业务上管什么”）

## 包含能力

（列模块对外提供的主要业务能力）

## 技术入口

（列真正值得作为入口的文件/函数。写“从哪里进、为什么看这里”，不是罗列整个目录）

## 关键约束

（写隐性约束、跨模块联动、业务坑点和容易误改的地方）

## 关联知识

（链接相关 concept/module/trouble）
```

合并规则：

- `模块职责`：保留旧描述，只补充新的高置信业务职责。
- `技术入口`：优先记录稳定函数、方法、类型或接口，并同步写入 `code_refs`；正文说明从哪里进入以及为什么。
- `包含能力`：合并去重。
- `关键约束`：优先保留旧的坑点和边界，再补充新的。
- `description`：如果模块职责、典型入口或影响面已经变化，必须同步改 frontmatter。
- `keywords`：如果模块新增别称、入口词、核心业务词或常见搜索词，必须同步更新。
- `code_refs`：按完整符号去重；只保存高置信当前入口，不保存行号或整条调用链。

### 6c：troubles

初始化阶段只保证目录存在，不从代码自动生成排查记录。

- 不创建 `README.md`
- 不伪造 trouble 文档
- 只在真实排查后由 `repomind-summary` 维护
- trouble 文档结构和每个小节含义，遵循 `repomind-summary` 中的排查模板说明

## 步骤 7：再次校验元数据并提交

写完文档后再次执行：

```bash
repomind kb-build
repomind kb-validate
git add .repomind/ graphify-out/
```

目的：

- 确认每个新文档都暴露了 `name` / `description`
- 确认没有回写旧的 `index.json` / README

## 步骤 8：输出摘要并衔接 PRD

输出初始化摘要时必须包含：

- 新建/合并了哪些 concepts
- 新建/合并了哪些 modules
- 待确认概念
- 待确认模块
- troubles 仍为空是预期行为

然后执行以下交互规则：

- 如果用户在最初请求里已经给了 PRD/需求文档路径，初始化完成后直接调用 `repomind-prd`，不要再问。
- 如果用户没有给路径，主动询问：

```text
如果你还有历史 PRD/需求文档，我可以继续补业务知识。把文档路径发给我即可；如果现在不需要，回复不用。
```

- 如果用户回复了路径，立即执行 `repomind-prd`，不要重新跑 init。
