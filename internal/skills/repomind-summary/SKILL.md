---
name: repomind-summary
description: 编码后总结。增量更新 graphify AST、分析业务影响范围、同步更新 .repomind/index.json 和 .repomind/modules/*.md。
---

# RepoMind 编码后更新

每次完成代码修改后，必须执行以下步骤，确保 RepoMind 知识库与代码保持同步。

## 步骤 1：增量更新图谱

先更新 graphify 的 AST 数据，只重提取变更文件：

```
/graphify --update
```

纯代码项目只走 AST（imports、calls、class、function），不调用 LLM，秒级完成。

## 步骤 2：查看改动

```bash
git diff --stat
git diff --name-only
```

确认本次修改涉及的所有文件。

## 步骤 3：分析业务影响范围

根据改动文件和内容，总结：

- **影响了哪些业务模块？**（对应 `.repomind/modules/*.md` 和 `.repomind/index.json` 中的模块）
- **是否是业务语义变化？**（新增业务能力、修改业务规则、废弃业务功能）
- **是否新增了关键代码入口？**（新的 service、handler、controller、api 等）
- **是否删除了关键代码？**（旧的入口文件被删除）

## 步骤 4：更新知识库

### 4a：更新 `.repomind/modules/*.md`

**需要更新：**
- 业务语义变化 → 更新「业务描述」「AI 注意事项」
- 新增业务入口 → 添加到「关键代码」，大文件（函数 > 3 个）需列出关键函数名
- 新增修改场景 → 更新「常见修改场景」，建议精确到函数名（如 `file.ts` 中的 `specificFunc()`）
- 新增业务模块 → 创建新的 `modules/<模块名>.md`

**关键代码粒度规则（同 repomind-init）：**
- 小文件（函数 ≤ 3 个）：列出文件路径和用途即可
- 大文件（函数 > 3 个）：必须在文件路径下列出关键函数名
- 判断依据：参考 `.repomind/graph/summary.json` 的 `symbols` 字段

**需要剔除：**
- 关键代码文件被删除 → 从「关键代码」列表中移除（含其下的函数名）
- 模块所有关键代码被删除 → 删除该 `modules/<模块名>.md`
- 修改场景不再适用 → 移除过时条目
- 某函数被删除 → 从「关键代码」对应文件的函数列表中移除

**不需要更新：**
- 格式化、变量重命名、简单重构
- 拆函数但不改变业务行为（新增的函数名应添加到关键代码列表中）
- 配置调整（非业务逻辑）

### 4b：同步更新 `.repomind/index.json`

当 `.repomind/modules/*.md` 发生变更时，同步更新 `index.json`：

- **新增模块** → 在 `modules` 数组中添加条目，填写 file、description、keywords（中英文）
- **修改模块** → 更新对应条目的 description、keywords
- **删除模块** → 从 `modules` 数组中移除对应条目

`index.json` 条目格式：

```json
{
  "file": "payment.md",
  "description": "支付核心模块，处理支付、退款、回调通知",
  "keywords": ["支付", "payment", "退款", "refund", "回调", "callback", "交易"]
}
```

## 步骤 5：输出变更摘要

```markdown
## RepoMind 更新摘要

### 修改的文件
- src/payment/payment.service.ts — 新增退款逻辑

### 影响的业务模块
- 支付模块 (payment)：更新了「常见修改场景」，新增退款相关入口

### index.json 同步
- payment 条目：更新了 keywords

### 已剔除的引用
- 无

### 新增/删除的模块文档
- 无
```

如果本次修改不需要更新知识库，明确说明原因。
