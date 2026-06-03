---
name: repomind-summary
description: 编码后、问答后或业务讨论后总结。读取 .repomind/.query-findings.json（如有），增量更新业务卡片、模块文档和 index.json。不限触发场景，有新业务知识就闭环。
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

如果有代码改动，调用 `graphify update .` 增量更新 AST。

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
| **显式概念定义** — 代码注释直接写明的定义 | "ProCharacterStatus 有 is_pro_character 字段" | ❌ 定义本身从 struct 就能看到 |
| **业务意图** — "为什么"这么做 | "为什么 getLeftVipEnd 不查 pay_order？是设计遗漏还是有意为之？" | ✅ 代码不会告诉你 |
| **历史上下文** — Bug 演进、版本差异、修复原因 | "初始版有 did!=lastLoginDid 翻倍 Bug，XX 日修复" | ✅ git log 不能直接串成因果链 |
| **隐性约束** — 跨模块联动、数据一致性保证 | "UserVipInfo 不查 assets，迁移补偿到 assets 的 VIP 在 App 端不可见" | ✅ 没有哪个单一文件能告诉你 |
| **排查路径** — SQL 组合、日志 grep 模式、判断决策树 | "查迁移先看 account_switch_records，再对 auth.pkg_alias 看是哪个账号" | ✅ 总结过的人踩过的坑 |
| **负数规则** — 它不做什么 | "getLeftVipEnd 不查 pay_order" | ✅ `not` 逻辑代码里不会标出来 |

**决策标准：如果一条信息可以在 30 秒内从代码或 git log 中直接读到，就不要写进知识库。**

## 步骤 4：更新知识库

### 4a：更新 `.repomind/glossary/*.md` 业务卡片

**这是新的一等知识层。** glossary 不再是单个 `glossary.md` 词典文件，而是一个目录，每个文件是一张业务卡片。

**什么时候新建/更新业务卡片：**
- 用户在问“某个功能/概念是什么、为什么有、和什么不同”
- 对话中确认了某个业务概念的定义、边界、历史原因
- 分析代码后发现“这不是黑话翻译，而是一套业务能力”

**卡片模板：**

```md
# 概念：Pro角色

## 是什么

（一句话业务定义）

## 为什么有

（业务目的）

## 用户可见表现

（前台感知）

## 核心规则

- ...

## 易混淆概念

- 不是 ...
- 区别于 ...

## 主模块

- `character`

## 关联模块

- `card`
- `llm`

## 关键代码

- `service/character_pro.go`
```

**业务卡片写作原则：**
- ✅ 写业务定义、存在目的、边界、混淆点、隐性规则、历史原因
- ✅ 主模块/关联模块/关键代码只作为落点辅助，不要喧宾夺主
- ❌ 不要把函数内部实现、SQL、表结构抄进卡片
- ❌ 不要把“代码里显式可见”的字段解释成业务知识

**命名建议：**
- 文件名用 kebab-case，例如 `pro-character.md`、`creator-level.md`
- 标题用中文业务概念，便于人读

### 4b：更新 `.repomind/modules/*.md`

**核心原则：永远追加或合并，不要整体覆盖。** 更新前先读取现有文档，只修改变动的部分，保留未涉及的内容。

**需要更新（只记代码不会告诉你的）：**
- **AI 注意事项** — 隐性约束、业务意图、排查要点、历史坑点（优先级最高，重点维护）
- **业务语义变化** — 新增能力、废弃功能、规则变更的**原因**（不是"改了什么"，而是"为什么改"）
- **关键入口** — 新增的入口函数名（只记文件路径 + 函数签名，不记具体实现逻辑）
- **关联的业务卡片引用** — 某个模块新出现了一个关键概念时，在模块文档里可补一句“详见 glossary/xxx.md”

**内容质量的负向检查（写完后再读一遍，删掉以下内容）：**
- ❌ 不要把函数内部逻辑写成伪代码或步骤列表（代码本身就是最好的文档）
- ❌ 不要把 SQL 语句写进文档（拷贝到注释都不如直接看代码）
- ❌ 不要把调用链写进文档（AI 的 code reading 能力可以直接 trace）
- ❌ 不要写"该文件负责 X"——文件名本身就有含义，文件路径列表在 index.json 就够了
- ✅ 优先写：边界条件、历史原因、跨模块联动、排查经验

**一句话原则：如果删掉这条信息后，AI 读代码依然能发现它 → 不要写。如果删掉后 AI 会踩坑 → 必须写。**

**需要剔除：**
- 关键代码文件被删除 → 从「关键代码」移除
- 模块所有关键代码被删除 → 删除该 `.md` 文件
- 修改场景不再适用 → 移除过时条目

### 4b：同步更新 `.repomind/index.json`

- **新增模块** → 在 `modules` 数组中添加条目（file/description/keywords）
- **修改模块** → 更新对应条目的 description/keywords
- **删除模块** → 从 `modules` 数组中移除

**关键词约束：**
- `keywords` 只放“模块判别词”，不要把泛业务词一股脑塞进去
- 概念解释交给 `.repomind/glossary/*.md`
- 如果某个词更适合回答“它是什么”，优先沉淀成业务卡片，不要勉强塞进模块 keywords

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

### 修改的业务卡片
- ...

### index.json 同步
- ...

### 新增/删除的模块文档
- ...
```

如果本次不需要更新知识库，明确说明原因。
