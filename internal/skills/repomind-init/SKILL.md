---
name: repomind-init
description: 初始化业务知识库。使用 /graphify 分析代码依赖关系，从零构建 .repomind/modules/*.md 和 .repomind/index.json。
---

# RepoMind 初始化知识库

当 `.repomind/index.json` 的 `modules` 数组为空，或 `.repomind/modules/` 下除 README.md 外无模块文档时，执行初始化。

> **重要**：以下 8 个步骤必须全部完成，缺一不可。graphify 安装和运行只是前置准备，步骤 5-8（浏览仓库、归纳模块、创建文档、输出摘要）才是初始化的核心产出。

## 步骤 1：检查并安装 graphify

先检查 graphify 是否已安装：

```bash
which graphify || command -v graphify
```

如果不存在，执行安装：

```bash
pip install graphifyy
```

安装后验证：

```bash
graphify --help
```

## 步骤 2：运行 /graphify

如果 `graphify-out/graph.json` 不存在（首次使用），运行：

```
/graphify .
```

项目如果是纯代码仓库，graphify 会自动检测并**只走 AST 提取**（imports、calls、class、function），不调用 LLM，零成本。仅当检测到文档/论文时才启用语义提取。输出在 `graphify-out/` 目录。

**graphify 是 RepoMind 的代码结构分析引擎**，提供文件间的依赖关系、社区聚类、入口节点等信息。RepoMind 在此之上做业务模块归纳。

`/graphify` 完成后，**继续执行步骤 3**，不要在此停止。

## 步骤 3：确保 graphify 核心文件被 git 跟踪

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

## 步骤 4：运行 graph-scan

```bash
.repomind/bin/repomind-internal graph-scan
```

这会读取 `graphify-out/graph.json`，生成 `.repomind/graph/summary.json`，获取：

- `module_candidates`：按目录推测的候选模块
- `entry_files`：识别出的入口文件
- `communities`：graphify 社区发现结果（如有）
- `symbols`：函数/方法级符号列表（name、file、pkg），用于判断文件粒度

## 步骤 5：浏览仓库结构

```bash
git ls-files
```

结合 graph summary 和 graphify 的 GRAPH_REPORT.md，理解：

- 顶层目录分布与业务对应关系
- 各目录下的文件组织
- 入口文件与 graphify 社区（community）的对应关系

## 步骤 6：归纳业务模块

根据以下信息综合判断业务模块：

1. **目录结构**：通常一个顶层目录对应一个业务模块
2. **graphify 社区发现**：代码依赖关系自然形成的社区往往对应业务边界
3. **入口文件**：controller、service、handler 等标识核心业务逻辑
4. **文件命名**：有明确业务含义的目录名/文件名

一个业务模块通常对应：
- 一个独立的业务领域（如：支付、订单、用户）
- 在代码中体现为一个或多个目录
- 有明确的入口文件

## 步骤 7：创建模块文档和索引

### 6a：创建 `.repomind/modules/<模块名>.md`

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

### 6b：创建 `.repomind/index.json`

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

### 6c：git add 知识库文件

将创建的模块文档和索引加入 git 版本控制：

```bash
git add .repomind/modules/ .repomind/index.json
```

这确保 RepoMind 知识库随代码一起提交，团队共享。

## 步骤 8：输出初始化摘要

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
