---
name: repomind-init
description: 初始化业务知识库。使用 graphify 全局分析代码依赖关系，从零构建 .repomind/modules/*.md 和 .repomind/index.json。Claude Code 通过 /graphify 触发，Codex 通过 $graphify 触发。
---

# RepoMind 初始化知识库

当 `.repomind/index.json` 的 `modules` 数组为空，或 `.repomind/modules/` 下除 README.md 外无模块文档时，执行初始化。

> **重要**：以下 7 个步骤必须全部完成，缺一不可。初始化是**全局重新生成**，不是增量更新，每次都会从头构建知识库。graphify 已由 `repomind install` 预先安装，可直接使用。

## 步骤 1：全局重新生成图谱（非增量）

这是 **全局重新生成**，不是增量更新。即使 `graphify-out/graph.json` 已存在也要重新跑，确保图谱与当前代码完全一致。

调用 graphify **skill**（注意：不是 shell 命令，不要按 bash 方式执行）进行全量分析：

- **如果你在 Claude Code 中**：输入 `/graphify .`（以 `/` 开头 = 调用 Claude Code skill）
- **如果你在 Codex 中**：输入 `$graphify .`（以 `$` 开头 = 调用 Codex skill）

> ⚠️ 这不是 shell 命令！不要在 bash/zsh 终端里运行 `/graphify .` 或 `$graphify .`。这些是 AI 编码助手内部的 skill 调用语法。

项目如果是纯代码仓库，graphify 会自动检测并**只走 AST 提取**（imports、calls、class、function），不调用 LLM，零成本。仅当检测到文档/论文时才启用语义提取。输出在 `graphify-out/` 目录。

**graphify 是 RepoMind 的代码结构分析引擎**，提供文件间的依赖关系、社区聚类、入口节点等信息。RepoMind 在此之上做业务模块归纳。

完成后，**继续执行步骤 3**，不要在此停止。

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
!modules/**
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

一个业务模块通常对应：
- 一个独立的业务领域（如：支付、订单、用户）
- 在代码中体现为一个或多个目录
- 有明确的入口文件

## 步骤 5.5：初始化业务黑话词典（glossary）

在创建模块文档之前，先分析代码库中的常见命名规律，生成初始业务黑话词典：

1. **收集字段命名**：从主要数据表（扫描 SQL DDL 或 model struct 文件）和 API 接口文档中提取字段名
2. **收集场景常量**：从 `buss_constants/contants.go` 等常量文件提取 scenes 名称
3. **构建映射表**：将中文业务口语转化为代码/DB 术语

初始 glossary 的生成原则：
- 优先从建表语句的 COMMENT 注释提取（如 `chat_bg` 注释为"聊天背景图片"）
- 优先从常量命名和注释提取（如 `discord_auto_review`）
- 不要编造映射关系，提取不到则留空，等待后续补充
- 参考 graphify 的 GRAPH_REPORT.md 中的高频概念

生成的 glossary.md 格式：
```markdown
# 业务黑话 ↔ 代码术语映射

## [领域名]

| 业务黑话 | 代码/DB 术语 | 说明 |
|---------|-------------|------|
| ... | ... | ... |
```

## 步骤 6：创建模块文档和索引（智能合并）

**重要：所有写入都是合并，不是覆盖。** 如果模块文档已存在，保留已有内容，只补充新的发现。

### 6a：创建/合并 `.repomind/modules/<模块名>.md`

对每个模块：
1. 先检查 `.repomind/modules/<模块名>.md` 是否已存在
2. **如果已存在**：读取现有文档，执行智能合并：
   - `业务描述`：保留原有描述，不修改
   - `关键代码`：合并新旧两份，去重（按文件路径去重）
   - `常见修改场景`：保留旧的，补充新的，去重
   - `AI 注意事项`：保留旧的，补充新的，去重
3. **如果不存在**：按以下模板新建

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

### 6b：创建/合并 `.repomind/index.json`

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

### 6c：git add 知识库文件

将创建的模块文档和索引加入 git 版本控制：

```bash
git add .repomind/modules/ .repomind/index.json
```

这确保 RepoMind 知识库随代码一起提交，团队共享。

## 步骤 7：输出初始化摘要

```markdown
## RepoMind 初始化完成

### 创建的模块文档
- payment.md — 支付模块
- order.md — 订单模块
- user.md — 用户模块

### index.json 已更新
- 登记了 N 个模块，共 M 个关键词

### 图谱分析摘要
- 模式: graphify / fallback
- 节点数: N
- 社区数: N
- 入口文件数: N
```
