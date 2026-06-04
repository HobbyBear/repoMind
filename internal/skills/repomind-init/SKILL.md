---
name: repomind-init
description: 初始化业务知识库。使用 graphify 全量重建代码图谱，保守归纳高置信业务概念与业务模块，合并写入 .repomind/concepts、modules 和 index.json。troubles 初始化只创建目录说明，不从代码生成排查记录。
---

# RepoMind 初始化知识库

当 `.repomind/index.json` 的 `modules` 数组为空，或 `.repomind/modules/` 下除 README.md 外无模块文档时，执行初始化。

> **重要**：以下 7 个步骤必须全部完成，缺一不可。初始化时 graphify 图谱必须**全量重新生成**，确保代码结构是当前事实；但 `.repomind/` 知识库写入必须采用**合并**，不能覆盖已有人工沉淀。初始化的目标是生成第一版可用知识种子，不是一次性写满完整业务手册。graphify 已由 `repomind install` 预先安装，可直接使用。

## 步骤 1：全局重新生成图谱（非增量）

这是 **全局重新生成**，不是增量更新。即使 `graphify-out/graph.json` 已存在也要重新跑，确保图谱与当前代码完全一致。

调用 graphify **skill**（注意：不是 shell 命令，不要按 bash 方式执行）进行全量分析：

- **如果你在 Claude Code 中**：输入 `/graphify .`（以 `/` 开头 = 调用 Claude Code skill）
- **如果你在 Codex 中**：输入 `$graphify .`（以 `$` 开头 = 调用 Codex skill）

> ⚠️ 这不是 shell 命令！不要在 bash/zsh 终端里运行 `/graphify .` 或 `$graphify .`。这些是 AI 编码助手内部的 skill 调用语法。

项目如果是纯代码仓库，graphify 会自动检测并**只走 AST 提取**（imports、calls、class、function），不调用 LLM，零成本。仅当检测到文档/论文时才启用语义提取。输出在 `graphify-out/` 目录。

**graphify 是 RepoMind 的代码结构分析引擎**，提供文件间的依赖关系、社区聚类、入口节点等信息。RepoMind 在此之上做业务模块归纳。

完成后，**继续执行步骤 2**，不要在此停止。

## 步骤 2：确保 graphify 核心文件被 git 跟踪

`graphify-out/` 下的 `.gitignore` 默认忽略所有文件，需要确认以下文件被 `!` 规则排除跟踪：

- `graphify-out/graph.json`
- `graphify-out/GRAPH_REPORT.md`
- `graphify-out/manifest.json`
- `graphify-out/graph.html`
- `graphify-out/.vocab.txt`

检查 `graphify-out/.gitignore` 是否包含上述 `!` 规则。缺失则补充：

```bash
cat >> graphify-out/.gitignore << 'EOF'
!manifest.json
!graph.html
!.vocab.txt
EOF
```

确认 `.repomind/.gitignore` 存在，确保 `index.json` 和 `modules/` 被跟踪：

```bash
ls .repomind/.gitignore || cat > .repomind/.gitignore << 'EOF'
!index.json
!concepts/**
!modules/**
!troubles/README.md
!bin/repomind-internal
EOF
```

所有文件加入 git 跟踪：

```bash
git add graphify-out/ .repomind/
```

## 步骤 3：运行 graph-scan

```bash
.repomind/bin/repomind-internal graph-scan
```

这会读取 `graphify-out/graph.json`，生成 `.repomind/graph/summary.json`，获取：

- `module_candidates`：按目录推测的候选模块
- `entry_files`：识别出的入口文件
- `communities`：graphify 社区发现结果（如有）
- `symbols`：函数/方法级符号列表（name、file、pkg），用于判断文件粒度

## 步骤 4：浏览仓库结构

```bash
git ls-files
```

结合 graph summary 和 graphify 的 GRAPH_REPORT.md，理解：

- 顶层目录分布与业务对应关系
- 各目录下的文件组织
- 入口文件与 graphify 社区（community）的对应关系

## 步骤 5：归纳业务模块

根据以下信息综合判断业务模块：

1. **目录结构**：通常一个顶层目录对应一个业务模块
2. **graphify 社区发现**：代码依赖关系自然形成的社区往往对应业务边界
3. **入口文件**：controller、service、handler 等标识核心业务逻辑
4. **文件命名**：有明确业务含义的目录名/文件名

先把候选目录分成三类：

| 类型 | 处理方式 | 例子 |
|------|----------|------|
| 业务模块 | 创建 `.repomind/modules/*.md` 并写入 `index.json` | payment、order、user、refund |
| 技术支撑模块 | 只有当它承载业务入口或跨模块业务约束时才写入 modules；否则作为相关业务模块的 AI 注意事项 | config、auth middleware、queue、scheduler |
| 忽略目录 | 不创建业务模块，不写入 index | scripts、test、docs、vendor、build、纯启动器 cmd、纯 utils |

一个业务模块通常对应：
- 一个独立的业务领域（如：支付、订单、用户）
- 在代码中体现为一个或多个目录
- 有明确的入口文件

不要把以下内容默认归纳为业务模块：
- 纯基础设施、通用工具、脚手架、测试目录
- 只有技术职责、没有业务语义的目录
- graphify 社区中因为依赖关系聚在一起、但没有业务边界的文件集合

如果不确定某个候选是否是业务模块，先放入初始化摘要的“待确认候选”，不要写入 `index.json`。

## 步骤 5.5：初始化业务概念卡片（concepts 目录）

在创建模块文档之前，先为仓库中最核心的业务概念生成初始业务卡片：

1. **收集高频业务概念**：从目录名、服务名、配置名、接口名、GRAPH_REPORT 高频概念中找“像业务能力”的对象
2. **提炼业务定义**：按新模板结构填充——它是什么、为什么有、用户侧表现、系统侧数据流
3. **识别边界和混淆点**：它不是什么、和哪个模块/概念容易混淆

初始业务卡片的生成原则：
- 只为**高置信业务概念**生成卡片；中低置信概念只列入初始化摘要的“待确认概念”
- 优先写”代码不容易直接表达的业务语义”，不要抄函数逻辑
- 优先选择高价值概念：核心角色类型、会员体系、推荐体系、审核体系、创作者体系
- 不要编造业务目的；如果“为什么有”无法从代码、README、接口语义中得到支撑，就留空或写“待确认”
- **描述要精确，不要笼统**。例如不要只写”AI 自动打标”，而是写明触发时机、数据来源、用户感知效果
- 参考 graphify 的 `GRAPH_REPORT.md` 中的高频概念，但不要把图谱社区名直接当业务定义

**高置信概念判断：**

| 置信度 | 判断标准 | 处理 |
|--------|----------|------|
| 高 | 概念在接口、服务、模型、配置、用户可见路径中反复出现，并能说明用户侧表现或系统侧数据流 | 生成/合并业务卡片 |
| 中 | 概念名称像业务能力，但只能从文件名或目录名推断 | 列入待确认，不生成卡片 |
| 低 | 只是技术名词、字段名、临时变量、内部工具名 | 丢弃 |

**从代码提取信息到卡片时做如下区分：**

| ✅ 提取到卡片（业务知识） | ❌ 不写进卡片（代码实现） |
|---|---|
| 触发时机（实时/定时/条件触发） | if/else 分支、switch 逻辑 |
| 用户侧感知效果 | 具体 SQL、表字段名、函数名 |
| 数据来源（谁产的数据） | 调用链、缓存策略 |
| 数据加工链路 | struct 定义、参数传递 |

生成的目录结构：

```text
.repomind/
  concepts/
    README.md
    pro-character.md
    creator-level.md
    ...
```

单张卡片格式：

```markdown
# 概念：XXX

## 是什么

## 为什么有

## 用户侧表现

（用户在什么场景看到什么、操作路径）

## 系统侧数据流

（数据在系统中如何产生、流转、最终表达）
- **数据来源**：谁产的？
- **处理链路**：经过什么步骤加工？
- **最终表达**：以什么形式被消费？

## 核心规则

（关键约束、边界条件，按子主题分组）

## 易混淆概念
```

## 步骤 6：创建模块文档和索引（智能合并）

**重要：所有知识库写入都是合并，不是覆盖。** 如果模块文档或业务卡片已存在，保留已有内容，只补充新的高置信发现。

### 6a：创建/合并 `.repomind/concepts/*.md`

对每个高置信业务概念：
1. 先检查 `.repomind/concepts/<概念名>.md` 是否已存在
2. **如果已存在**：保留原有定义和边界描述，只补充新增的业务语义、混淆点、关键规则
3. **如果不存在**：按步骤 5.5 的模板新建
4. 中低置信概念只输出到初始化摘要的“待确认概念”，不要写文件

### 6b：创建/合并 `.repomind/modules/<模块名>.md`

只对业务模块创建或合并文档：
1. 先检查 `.repomind/modules/<模块名>.md` 是否已存在
2. **如果已存在**：读取现有文档，执行智能合并：
   - `业务描述`：保留原有描述，不修改
   - `关键代码`：合并新旧两份，去重（按文件路径去重）
   - `常见修改场景`：保留旧的，补充新的，去重
   - `AI 注意事项`：保留旧的，补充新的，去重
3. **如果不存在**：按以下模板新建
4. 技术支撑模块只有在承载业务入口或跨模块业务约束时才单独建文档；否则写到相关业务模块的 AI 注意事项

```md
# 模块名称

## 业务描述

（简洁描述该模块负责的核心业务，1-3 句话）

## 关键代码

- `path/to/service.ts`
  - 核心业务逻辑入口
  - 关键函数：`createOrder()`, `cancelOrder()`, `refund()`

- `path/to/handler.ts`
  - 外部请求处理入口

## 常见修改场景

- 修改某功能：优先查看 `path/to/file.ts` 中的 `specificFunc()`

## AI 注意事项

- 需要注意的业务约束或编码规范
```

**关键代码粒度规则：**

- **小文件（函数 ≤ 3 个）**：只需列出文件路径和用途说明，无需列函数名
- **大文件（函数 > 3 个）**：必须在文件路径下列出与该模块相关的关键函数名，方便后续通过函数名定位代码
- **判断依据**：参考 `summary.json` 的 `symbols` 字段，按 file 聚合即可得到每个文件的函数数量
- 函数名包含方法接收者（如 `*Service.CreateOrder`），便于区分同名方法

### 6c：创建/合并 `.repomind/index.json`

**不要直接覆盖 index.json。** 先读取现有文件（如果存在），合并后写入：

```json
{
  "modules": [
    {
      "file": "payment.md",
      "description": "支付核心模块，处理支付、退款、回调通知",
      "keywords": ["支付", "payment", "退款", "refund", "回调", "callback", "交易"]
    }
  ]
}
```

**合并规则：**
1. 读取现有的 `.repomind/index.json`，解析 `modules` 数组
2. 对每个新模块，检查 `file` 是否已在现有数组中
3. 已存在的模块：保留原条目不动（不覆盖 description/keywords）
4. 不存在的模块：追加到数组末尾
5. 写回 index.json

每个模块条目：
- `file`：对应的 `.repomind/modules/` 下的文件名
- `description`：一句话业务描述（中文）
- `keywords`：中英文关键词数组，用于快速搜索定位

**关键词约束：**
- 只放模块判别词，不堆泛业务词
- 概念解释交给 `.repomind/concepts/*.md`
- 低置信候选不写入 `index.json`

### 6d：初始化 `.repomind/troubles/`

初始化阶段只创建排查记录目录和 README，不从代码图谱生成 trouble 记录。

原因：`troubles` 记录的是历史排查经验，必须来自真实问题、排查过程、根因和验证路径；仅靠代码结构无法可靠生成。

如果 `.repomind/troubles/README.md` 不存在，创建目录说明；如果已存在，保持不变。

README 内容建议：

```markdown
# RepoMind 排查记录

本目录记录真实问题排查后的可复用经验。

只写来自实际排查的问题现象、判断路径、根因、验证方式和坑点。
不要从初始化图谱、静态代码结构或猜测中生成排查记录。
```

### 6e：git add 知识库文件

将创建的模块文档和索引加入 git 版本控制：

```bash
git add .repomind/concepts/ .repomind/modules/ .repomind/index.json .repomind/troubles/README.md
```

这确保 RepoMind 知识库随代码一起提交，团队共享。

## 步骤 7：输出初始化摘要

```markdown
## RepoMind 初始化完成

### 创建的模块文档
- payment.md — 支付模块
- order.md — 订单模块
- user.md — 用户模块

### 创建的业务卡片
- pro-character.md — Pro 角色
- creator-level.md — 创作者等级

### 待确认候选
- 概念：xxx — 置信度中，原因：仅从目录名推断
- 模块：xxx — 置信度中，原因：可能是技术支撑模块

### index.json 已更新
- 登记了 N 个模块，共 M 个关键词

### troubles 初始化
- 已创建/保留 `.repomind/troubles/README.md`
- 未从代码图谱生成排查记录

### 图谱分析摘要
- 模式: graphify / fallback
- 节点数: N
- 社区数: N
- 入口文件数: N
```
