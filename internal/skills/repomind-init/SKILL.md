---
name: repomind-init
description: 初始化业务知识库。使用 /graphify 分析代码依赖关系，从零构建 .repomind/modules/*.md 和 .repomind/index.json。
---

# RepoMind 初始化知识库

当 `.repomind/index.json` 的 `modules` 数组为空，或 `.repomind/modules/` 下除 README.md 外无模块文档时，执行初始化。

## 步骤 1：运行图谱分析

### 1a：运行 graph-scan

```bash
.repomind/bin/repomind-internal graph-scan
```

这会尝试读取 `graphify-out/graph.json`（如果 graphify 已运行过）。读取 `.repomind/graph/summary.json` 获取：

- `module_candidates`：按目录推测的候选模块
- `entry_files`：识别出的入口文件
- `communities`：graphify 社区发现结果（如有）

### 1b：如果 graphify-out/ 不存在，运行 /graphify

如果 `graphify-out/graph.json` 不存在（首次使用），运行：

```
/graphify .
```

项目如果是纯代码仓库，graphify 会自动检测并**只走 AST 提取**（imports、calls、class、function），不调用 LLM，零成本。仅当检测到文档/论文时才启用语义提取。输出在 `graphify-out/` 目录。

**graphify 是 RepoMind 的代码结构分析引擎**，提供文件间的依赖关系、社区聚类、入口节点等信息。RepoMind 在此之上做业务模块归纳。

## 步骤 2：浏览仓库结构

```bash
git ls-files
```

结合 graph summary 和 graphify 的 GRAPH_REPORT.md，理解：

- 顶层目录分布与业务对应关系
- 各目录下的文件组织
- 入口文件与 graphify 社区（community）的对应关系

## 步骤 3：归纳业务模块

根据以下信息综合判断业务模块：

1. **目录结构**：通常一个顶层目录对应一个业务模块
2. **graphify 社区发现**：代码依赖关系自然形成的社区往往对应业务边界
3. **入口文件**：controller、service、handler 等标识核心业务逻辑
4. **文件命名**：有明确业务含义的目录名/文件名

一个业务模块通常对应：
- 一个独立的业务领域（如：支付、订单、用户）
- 在代码中体现为一个或多个目录
- 有明确的入口文件

## 步骤 4：创建模块文档和索引

### 4a：创建 `.repomind/modules/<模块名>.md`

```md
# 模块名称

## 业务描述

（简洁描述该模块负责的核心业务，1-3 句话）

## 关键代码

- `path/to/service.ts`
  - 核心业务逻辑入口

- `path/to/handler.ts`
  - 外部请求处理入口

## 常见修改场景

- 修改某功能：优先查看 `path/to/file.ts`

## AI 注意事项

- 需要注意的业务约束或编码规范
```

**只记录业务描述和关键代码入口**，不要记录函数列表、调用链。

### 4b：创建 `.repomind/index.json`

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

每个模块条目：
- `file`：对应的 `.repomind/modules/` 下的文件名
- `description`：一句话业务描述（中文）
- `keywords`：中英文关键词数组，用于快速搜索定位

## 步骤 5：输出初始化摘要

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
