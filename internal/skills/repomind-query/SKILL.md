---
name: repomind-query
description: 编码前查询。理解用户需求后，先扩写需求、提取关键词组、查询 RepoMind 知识库定位相关代码，必要时使用 /graphify 深入分析代码依赖。
---

# RepoMind 编码前分析

在理解用户需求、查看代码之前，必须先完成以下步骤。

## 步骤 1：读取模块索引

读取 `.repomind/index.json`，获取所有已登记的业务模块概览（描述、关键词）。

`index.json` 格式：

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

## 步骤 2：扩写需求

将用户的原始需求扩写为更完整的业务描述：

- 用户想要达成什么业务目标？
- 涉及哪些业务领域或模块？
- 可能影响的上下游业务是什么？

## 步骤 3：提取关键词组

根据扩写后的需求，列出 **2-4 组关键词**，每组包含中英文、同义词、相关术语：

```
关键词组 1: 支付, payment, 交易, transaction
关键词组 2: 回调, callback, webhook, 通知
关键词组 3: 退款, refund, 逆向流程
```

## 步骤 4：查询知识库

### 4a：用关键词匹配 index.json

用每组关键词匹配 `.repomind/index.json` 中各模块的 `keywords` 和 `description` 字段，快速定位候选模块。

### 4b：深入阅读模块文档

读匹配到的 `.repomind/modules/<file>`，获取详细业务上下文、关键代码、修改场景和 AI 注意事项。

### 4c：使用 /graphify 深入分析

**必须执行**。使用 `/graphify query "<问题>"` 查询代码图谱，获取代码间的调用关系、依赖链路、社区结构等信息。

`/graphify` 从 `graphify-out/graph.json` 读取图谱数据（该文件由 graphify skill 生成和管理）。即使模块文档已包含关键代码路径，graphify 也能补充跨模块调用链和数据流，这些是静态文档无法覆盖的。

## 步骤 5：列出代码位置及含义

根据模块文档和 graphify 分析结果，整理清单：

```markdown
## 需求扩写
（完整业务描述）

## 关键词组
- 组1: ...
- 组2: ...

## 相关模块及代码

### 模块：支付 (payment)
- 业务描述：...
- 关键代码：
  - `src/payment/payment.service.ts` — 支付核心逻辑
  - `src/payment/payment-callback.ts` — 第三方回调入口
- 常见修改场景：...
- AI 注意事项：...

### 图谱分析
- 调用关系：...
- 关键依赖：...
```

**这份清单必须在开始编码前完成。** 如果 `.repomind/index.json` 为空，建议先执行 repomind-init 初始化知识库。
