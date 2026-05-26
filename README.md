# RepoMind

Skill-first 的本地业务代码知识库系统。

## 定位

RepoMind 不是传统 CLI，不要求用户日常手动执行 query/update/init。

用户只需要在代码仓库里执行一次 `repomind install`，之后日常使用由 Claude Code / Codex 的 skill 自动完成。

## 核心理念

```text
Skill-first — 由 AI skill 驱动，而非用户手动操作
Zero-config — 零配置，不需要 config.yaml
Markdown 即真相 — .repomind/modules/*.md 是长期业务知识源
index.json — 模块索引，含描述和关键词，快速定位模块
图谱驱动 — 集成 graphify skill，代码依赖关系 + 社区发现辅助模块归纳
AI 维护 — 业务知识库由 Claude/Codex skill 创建和维护
```

## 维护什么

```text
index.json        — 模块索引（描述、关键词）
modules/*.md      — 业务描述 + 关键代码入口
graph/            — graphify 分析缓存
```

## 不维护什么

```text
symbols.json / flow.json / routes.json
完整调用链 / 函数级映射 / 依赖图
```

## 安装

```bash
go build -o repomind ./cmd/repomind/
./repomind install
```

## 安装后的目录结构

```text
.repomind/
  index.json        # 模块索引（描述、关键词）
  modules/          # 业务模块文档（Markdown）
    README.md
  graph/            # 图谱分析缓存
    summary.json
  bin/              # 内部工具
    repomind-internal

.claude/skills/
  repomind-query/SKILL.md
  repomind-summary/SKILL.md
  repomind-init/SKILL.md

.codex/skills/
  repomind-query/SKILL.md
  repomind-summary/SKILL.md
  repomind-init/SKILL.md

graphify-out/       # graphify skill 输出（由 /graphify 生成）
  graph.json
  GRAPH_REPORT.md
  ...
```

## 三个 Skill

### Skill 1: 编码前查询（repomind-query）

理解需求后、看代码前：
1. 读取 `.repomind/index.json` 获取模块索引
2. 扩写需求，提取 2-4 组中英文关键词
3. 用关键词匹配 index.json 和搜索 modules/*.md
4. 必要时使用 `/graphify query` 深入分析代码依赖
5. 列出具体代码位置及其业务含义

### Skill 2: 编码后总结（repomind-summary）

每次写完代码后：
1. 用 `git diff` 查看改动
2. 分析业务影响范围
3. 更新 `.repomind/modules/*.md`（新增/修改/删除）
4. 同步更新 `.repomind/index.json`
5. 剔除已删除文件的引用

### Skill 3: 初始化知识库（repomind-init）

模块索引为空时：
1. 运行 `/graphify .` 构建代码知识图谱
2. 运行 `graph-scan` 获取图谱摘要
3. 结合 graphify 社区发现和目录结构归纳业务模块
4. 创建 `.repomind/modules/*.md` 和 `.repomind/index.json`

## 用户命令

只暴露一个公开命令：

```bash
repomind install
```

## 内部命令（由 skill 调用）

```bash
.repomind/bin/repomind-internal graph-scan    # 读取 graphify 输出，生成摘要
```

## 工作流程

```text
编码前 → Skill 1: index.json 快速定位 → modules/*.md 深入理解 → /graphify query 追踪依赖
编码中 → 参考模块文档理解业务上下文
编码后 → Skill 2: git diff → 分析影响 → 更新 modules + index.json
首次使用 → Skill 3: /graphify → graph-scan → 创建 modules + index.json
```

## 模块文档格式

```md
# 模块名称

## 业务描述

## 关键代码

## 常见修改场景

## AI 注意事项
```

## index.json 格式

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

## 项目结构

```text
cmd/repomind/main.go
internal/
  cli/          # CLI 命令（install + graph-scan）
  fsutil/       # 文件系统工具
  gitutil/      # Git 集成
  graph/        # 图谱分析（读取 graphify 输出 + fallback scanner）
  skills/       # Skill 文件嵌入
    skills.go
    repomind-query/SKILL.md
    repomind-summary/SKILL.md
    repomind-init/SKILL.md
```
