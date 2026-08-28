# RepoMind

为 Claude Code / Codex 编码助手提供业务代码知识库。AI 在修改代码前自动查询业务卡片和相关模块、理解上下文，编码后自动更新知识库，确保每次改动都有据可查。

![img.png](img.png)

## 核心特性

- **零手动** — 安装后 AI 自动在编码前后查询和更新知识库，开发者无需记忆任何命令
- **直接路由** — Skill 直接读取 `name/description/keywords` 选择知识文档，不依赖额外查询命令
- **多维知识** — 业务卡片（concepts）定义"是什么"、模块文档（modules）定位"在哪改"、排查记录（troubles）沉淀"为什么出错"
- **人工可维护** — 产品、运营直接修改简洁 Markdown；机器目录和首页由 RepoMind 自动生成
- **写后校验** — summary 写入后严格检查格式、关键词和文档体积
- **压缩把关** — 自动检查关键词数量、文件和章节体积，达到阈值后要求压缩或拆分
- **Git 自动同步** — 提交前增量更新图谱，知识库始终与代码状态一致
- **智能 Gate** — 编码后先做轻量判断，有实质变更才写入，防止碎片知识污染
- **PRD 提取** — 从需求文档自动提取业务概念，需求阶段即可沉淀知识
- **格式自迁移** — 兼容旧版知识文件，安装时自动升级到最新元数据格式

## 安装

### 从源码构建

```bash
make build      # 本地编译（静态链接，无 glibc 依赖）
make build-all  # 交叉编译所有平台
```

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/HobbyBear/repoMind/master/install.sh | bash
```

自动识别平台，安装到 `/usr/local/bin` 或 `~/.local/bin`。

### Windows（PowerShell）

```powershell
powershell -c "iwr -useb https://raw.githubusercontent.com/HobbyBear/repoMind/master/install.ps1 | iex"
```

管理员运行安装到 `C:\ProgramData\repomind\bin`（系统 PATH），普通用户安装到 `%LOCALAPPDATA%\repomind\bin`（用户 PATH）。

### 初始化知识库

```bash
cd your-project
repomind install
```

自动完成：创建知识库目录 → 安装 Claude Code / Codex skill → 配置 AI 规则与 git hook → 迁移旧格式 → 生成知识目录。

## 使用

安装后无需手动操作，AI 自动执行：

- **编码前** — `repomind-query`：通过元数据匹配业务模块、定位关键代码
- **编码后** — `repomind-summary`：智能判断是否有新知识需沉淀，按需更新
- **随手记** — 用户说"记一下 / 总结到知识库"，自动分类写入 concepts / modules / troubles
- **PRD 处理** — `repomind-prd`：从需求文档提取业务概念

首次安装后知识库为空，在 Claude Code 中执行 `/repomind-init`（Codex 中 `$repomind-init`），AI 会自动运行 graphify 构建图谱并初始化业务卡片。

`repomind-query` 和 `repomind-summary` 的唯一源码位于 RepoMind，并被编译进二进制。FixForge 等外部系统只调用已部署的 skill，负责页面、对话和权限；不复制检索或总结规则。

## 知识目录

```text
.repomind/
├── README.md              # 自动生成的人类导航页
├── concepts/              # 人工维护：业务概念和规则
├── modules/               # 人工 + 工程维护：模块能力和技术入口
├── troubles/              # 人工 + 工程维护：故障排查和数据查询
└── .generated/
    └── catalog.json       # 自动生成的机器目录，不直接修改
```

人工文档只要求少量可见 frontmatter：

```yaml
---
name: "VIP 会员"
description: "VIP 订阅权益。判断购买、生效、续费和权益边界时查看。"
status: active
keywords: ["VIP", "会员", "订阅"]
---
```

`status: draft` 可以保存未完成内容，默认路由不会读取；发布为 `active` 后才进入正常知识源。
已作废但仍有历史价值的内容使用 `status: deprecated`，默认路由同样排除；只有用户明确要求检查历史结论时才读取。

固定正文格式如下：

| 类型 | 必备章节 | 常用可选章节 |
|---|---|---|
| `concept` | `这是什么`、`核心规则` | `适用场景与边界`、`关联知识` |
| `module` | `模块职责`、`包含能力`、`技术入口` | `关键约束`、`关联知识` |
| `trouble` | `问题现象`、`排查方法`、`结果判断` | `数据查询`、`根因与处理`、`关联知识` |

旧标题仍可读取，`kb-validate` 会提示逐步迁移，不会因为一次升级整体覆盖人工正文。

## 外部调用

外部系统把 RepoMind 当作数据源时，使用以下稳定入口：

```bash
repomind kb-build
repomind kb-audit
repomind kb-validate
repomind kb-new --kind trouble --name "充值订单一直处理中" \
  --description "充值订单长时间处于处理中时查看" \
  --keywords "充值,订单,支付回调" --status draft
```

- `kb-build`：规范化人工文档并生成目录和首页。
- `kb-audit`：审计整库体积、超长页面和路由元数据。
- `kb-validate`：检查格式、关键词和体积限制；`--file` 只验收本次文件，`--strict` 将 warning 也视为失败。
- `kb-new`：生成产品、运营可直接填写的固定模板。

summary 写入后必须对本次文件执行 `kb-validate --strict --file <文件>`，通过后才能完成写入流程。

## 压缩规则

- 关键词建议 3-8 个；超过 8 个触发警告。
- 单个关键词超过 32 个字符触发警告。
- 文件超过 8 KiB 或单章节超过 2 KiB，触发拆分建议。
- 文件超过 12 KiB 或单章节超过 4 KiB，校验失败，必须删除日志式内容或按独立业务主题拆分。
- 文件超过 150 行触发可读性警告，优先移走个案、修订流水和长故障过程。
- `.generated/` 仅保存可重建数据，业务事实始终以人工可见 Markdown 为准。

## 命令

```bash
repomind install      # 初始化知识库
repomind uninstall    # 移除
repomind update       # 更新到最新版本
repomind kb-build     # 生成目录与人类首页
repomind kb-audit     # 审计整库人可读性
repomind kb-validate  # 校验知识质量
repomind kb-new       # 按模板新增知识
repomind compact-prompt # 输出压缩 Skill，供外部系统调用
```

压缩由 `repomind-compact` Skill 直接识别用户给出的范围。指定文件或目录时只读取、合并和校验这些文件；没有指定范围时才执行整库精简：

```text
$repomind-compact 合并并精简 .repomind/troubles/a.md 和 .repomind/troubles/b.md
$repomind-compact 只整理 .repomind/troubles/payment/ 目录
```

`repomind update` 下载最新发布二进制后，会用其中内置的官方 Skill 整目录覆盖项目里的旧版本，并删除旧版本遗留文件。

## 与Graphify及类似的知识库的区别

[Graphify](https://github.com/HobbyBear/graphify) 是代码结构分析引擎——扫描 AST 生成依赖图和社区聚类，回答**"代码在技术上怎样组织"**。纯静态分析，不涉及业务语义。

RepoMind 在 Graphify 之上叠加业务语义层，将技术图谱转化为 AI 可消费的业务文档，回答**"这个概念是什么、这个模块干什么、改它注意什么"**。

> 类比：Graphify 是建筑结构图（承重墙在哪、管道怎么走），RepoMind 是房间功能卡（这是厨房、插座位置、注意防水）。

两者互补：AST 提取不出业务语义，而 AI 改代码需要同时知道业务含义和代码位置——前者靠 RepoMind 沉淀，后者靠 Graphify 定位。
