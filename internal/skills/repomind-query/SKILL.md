---
name: repomind-query
description: 查阅任何代码/业务逻辑时优先自动触发，查询 RepoMind 知识库 + graphify 图谱定位关键代码，回答后自动保存发现供 repomind-summary 更新知识库。
---

# RepoMind 编码前/问答分析

**任何涉及代码、业务逻辑、项目结构的提问，都必须先执行本流程。** 不要在未查询知识库的情况下直接回答。

## 步骤 1：读取模块索引

读取 `.repomind/index.json`，获取所有已登记的业务模块概览（描述、关键词）。

## 步骤 2：理解用户意图并扩写

- 用户真正想问什么？（代码位置？业务逻辑？调用链？）
- 涉及哪些业务领域或模块？
- 可能影响的上下游是什么？

## 步骤 3：提取关键词组

根据扩写后的意图，列出 **2-4 组关键词**（中英文、同义词、相关术语）。

## 步骤 4：查询知识库

### 4a：关键词匹配 index.json

用每组关键词匹配 `.repomind/index.json` 中各模块的 `keywords` 和 `description` 字段。

### 4b：深入阅读模块文档

读匹配到的 `.repomind/modules/<file>`，获取详细业务上下文、关键代码、修改场景和 AI 注意事项。

### 4c：使用 graphify skill 深入分析

**必须执行**。调用 `/graphify query "<问题>"` 查询代码图谱：
- 即使模块文档已有关键代码路径，graphify 也能补充跨模块调用链和数据流

## 步骤 5：整理代码位置及含义

按以下格式输出查询结果。**这份清单必须在回答前完成：**

```markdown
## 知识库查询结果

### 用户意图
（一句话说清用户想知道的）

### 关键词组
- 组1: ...

### 相关模块及代码

#### 模块：xxx (xxx.md)
- 业务描述：...
- 关键代码：
  - `路径/文件.go:行号` — 用途
- AI 注意事项：...

### 图谱补充
- 调用关系：...
```

## 步骤 6：保存查询发现

将本次查询中**超出已有知识库的新发现**（新接口、新逻辑、新代码位置、模块边界更新）写入临时文件，自动触发后续知识库更新：

**自动判定规则：**
- `new_findings` 数组**不为空** → `needs_summary: true`（有新知识需要同步）
- `new_findings` 数组**为空**（所有信息已在模块文档中）→ `needs_summary: false`

```bash
cat > .repomind/.query-findings.json << 'JSONEOF'
{
  "trigger": "问答",
  "intent": "用户意图简述",
  "known_modules": ["已命中模块名"],
  "new_findings": [
    {
      "type": "new_code_location|new_business_rule|module_update",
      "module": "模块名",
      "file": "路径",
      "content": "发现描述"
    }
  ],
  "needs_summary": true|false
}
JSONEOF
```

> `needs_summary` **必须**根据 `new_findings` 数组是否为空自动判定，不要手动预设 `false`。
> 即使发现信息和已有知识库部分重叠，只要有任何一条全新信息，就设为 `true`。

## 步骤 7：自动更新知识库

如果 `needs_summary == true`（即 new_findings 不为空），**自动调用 repomind-summary skill** 完成知识库更新，无需等待用户指令：

```
Skill: repomind-summary
```

> 这样确保 "查询 → 发现 → 存储" 全流程闭环，新知识不会因为忘记执行总结而被遗漏。
