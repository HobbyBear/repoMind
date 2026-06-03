---
name: repomind-summary
description: 编码后、问答后或业务讨论后总结。读取 .repomind/.query-findings.json（如有），增量更新模块文档和 index.json。不限触发场景，有新业务知识就闭环。
---

# RepoMind 编码/问答/业务讨论后更新

当以下**任一**场景完成时，必须执行本流程：
- 修改了代码（编码后）
- 回答了代码/业务问题且有新发现（`.repomind/.query-findings.json` 存在且 `needs_summary = true`）
- **在业务讨论/排查/需求分析中发现了超出已有知识库的知识**（没有 query-findings 文件时，手动编写发现内容）

## 步骤 0：读取查询发现（如有）

```bash
cat .repomind/.query-findings.json 2>/dev/null || echo '{"needs_summary": false}'
```

如果 `needs_summary = true`，将发现中的内容作为本次更新的输入。

## 步骤 0.5：无 query-findings 时的处理

如果 `.repomind/.query-findings.json` 不存在（即本次总结不是由 `repomind-query` 触发的，而是在业务讨论/排查中自行发现的新知识），你需要：

1. **回顾整个对话**，找出所有超出已有知识库的新知识
2. **自行编写发现内容**，遵循 query-findings 的格式写入临时文件：

```bash
cat > .repomind/.query-findings.json << 'JSONEOF'
{
  "trigger": "业务讨论",
  "intent": "描述发现了什么",
  "known_modules": ["涉及模块"],
  "new_findings": [
    {
      "type": "new_business_rule|new_code_location|module_update",
      "module": "模块名",
      "file": "路径",
      "content": "发现描述"
    }
  ],
  "needs_summary": true
}
JSONEOF
```

3. 然后继续正常流程（步骤 1-5）

## 步骤 1：增量更新图谱（仅编码后）

如果有代码改动，调用 `/graphify --update` 增量更新 AST。

纯代码项目只走 AST，秒级完成。

## 步骤 2：查看改动

```bash
git diff HEAD --stat
git diff HEAD --name-only
```

如果是从问答发现来的更新（无代码改动），跳过此步。

## 步骤 3：分析业务影响范围（先判断什么该记、什么不该记）

### 核心原则：只记"代码不会告诉你的东西"

在决定更新知识库之前，先严格区分：

| 类别 | 举例 | 该不该记？ |
|------|------|-----------|
| **代码结构** — 文件结构、函数名、函数参数、调用链 | "MoveAccount 先 INSERT 再 DELETE 再 UPDATE" | ❌ 代码就在那，AI 直接读就行 |
| **表字段** — 字段名、类型、注释 | "assets 表有 ultra_vip_sec 字段" | ❌ DDL 或 model struct 一目了然 |
| **函数逻辑** — if/else 分支、循环、具体 SQL | "getLeftVipEnd 查 subscription_order WHERE did=?" | ❌ 看源码比看文档快 |
| **业务意图** — "为什么"这么做 | "为什么 getLeftVipEnd 不查 pay_order？是设计遗漏还是有意为之？" | ✅ 代码不会告诉你 |
| **历史上下文** — Bug 演进、版本差异、修复原因 | "初始版有 did!=lastLoginDid 翻倍 Bug，XX 日修复" | ✅ git log 不能直接串成因果链 |
| **隐性约束** — 跨模块联动、数据一致性保证 | "UserVipInfo 不查 assets，迁移补偿到 assets 的 VIP 在 App 端不可见" | ✅ 没有哪个单一文件能告诉你 |
| **排查路径** — SQL 组合、日志 grep 模式、判断决策树 | "查迁移先看 account_switch_records，再对 auth.pkg_alias 看是哪个账号" | ✅ 总结过的人踩过的坑 |
| **负数规则** — 它不做什么 | "getLeftVipEnd 不查 pay_order" | ✅ `not` 逻辑代码里不会标出来 |

**决策标准：如果一条信息可以在 30 秒内从代码或 git log 中直接读到，就不要写进知识库。**

## 步骤 4：更新知识库

### 4a：更新 `.repomind/modules/*.md`

**核心原则：永远追加或合并，不要整体覆盖。** 更新前先读取现有文档，只修改变动的部分，保留未涉及的内容。

**需要更新（只记代码不会告诉你的）：**
- **AI 注意事项** — 隐性约束、业务意图、排查要点、历史坑点（优先级最高，重点维护）
- **业务语义变化** — 新增能力、废弃功能、规则变更的**原因**（不是"改了什么"，而是"为什么改"）
- **关键入口** — 新增的入口函数名（只记文件路径 + 函数签名，不记具体实现逻辑）
- **glossary 业务黑话** — 中文口语 ↔ 代码术语映射（只记映射关系，不记表结构）

**内容质量的负向检查（写完后再读一遍，删掉以下内容）：**
- ❌ 不要把函数内部逻辑写成伪代码或步骤列表（代码本身就是最好的文档）
- ❌ 不要把 SQL 语句写进文档（拷贝到注释都不如直接看代码）
- ❌ 不要把调用链写进文档（AI 的 code reading 能力可以直接 trace）
- ❌ 不要写"该文件负责 X"——文件名本身就有含义，文件路径列表在 index.json 就够了
- ✅ 优先写：边界条件、历史原因、跨模块联动、排查经验

**一句话原则：如果删掉这条信息后，AI 读代码依然能发现它 → 不要写。如果删掉后 AI 会踩坑 → 必须写。**

**业务黑话（glossary）更新：**
如果本次问答发现中包含了「待补充业务黑话」标记，或有新的业务术语 ↔ 代码字段映射关系被确认，则：
1. 检查 `.repomind/modules/glossary.md` 是否存在，不存在则创建
2. 在其中新增对应条目（业务黑话 ↔ 代码/DB 术语 + 说明）
3. 同时在 `.repomind/index.json` 中确保 glossary 模块的 keywords 包含新增术语

glossary.md 的格式规范：
- 按业务领域分表格（如# 人设相关、# 审核相关、# 模型配置相关）
- 每行一个映射：| 业务黑话 | 代码/DB 术语 | 说明 |
- 说明要简洁，让 AI 能直接定位到代码

**需要剔除：**
- 关键代码文件被删除 → 从「关键代码」移除
- 模块所有关键代码被删除 → 删除该 `.md` 文件
- 修改场景不再适用 → 移除过时条目

### 4b：同步更新 `.repomind/index.json`

- **新增模块** → 在 `modules` 数组中添加条目（file/description/keywords）
- **修改模块** → 更新对应条目的 description/keywords
- **删除模块** → 从 `modules` 数组中移除

### 4c：清理临时文件

```bash
rm -f .repomind/.query-findings.json
```

## 步骤 5：输出变更摘要

```markdown
## RepoMind 更新摘要

### 来源
- [编码后] 或 [问答后]

### 修改的文件
- ...

### 影响的业务模块
- ...

### index.json 同步
- ...

### 新增/删除的模块文档
- ...
```

如果本次不需要更新知识库，明确说明原因。
