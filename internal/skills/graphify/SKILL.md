---
name: graphify
description: 代码知识图谱分析。构建代码依赖关系图谱，支持社区发现、调用链查询。当用户需要分析代码结构、依赖关系、模块划分时使用。
---

# Graphify 代码图谱分析

基于 AST 提取构建代码知识图谱，支持依赖分析、社区发现、入口识别。

## 命令

```
/graphify .              # 全量分析当前目录
/graphify --update       # 增量更新（只重提取变更文件）
/graphify query "<问题>"  # 自然语言查询图谱
```

## 工作流程

1. 扫描源码 AST，提取 imports、calls、functions、classes
2. 构建依赖图，运行社区发现算法
3. 输出到 `graphify-out/`（`graph.json`、`GRAPH_REPORT.md` 等）

纯代码项目只走 AST，不调用 LLM，零成本。
