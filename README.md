# RepoMind

为 Claude Code / Codex 编码助手提供业务代码知识库。AI 在修改代码前自动查询相关业务模块、理解上下文，编码后自动更新知识库，确保每次改动都有据可查。

## 安装

```bash
# macOS / Linux
curl -fsSL https://github.com/HobbyBear/repoMind/releases/latest/download/install.sh | bash
```

安装后进入项目目录，初始化知识库：

```bash
cd your-project
repomind install
```

`repomind install` 会自动完成：

- 创建 `.repomind/` 知识库目录（模块文档 + 索引 + 图谱缓存）
- 安装 Claude Code skill（`.claude/skills/repomind-*`）和 Codex skill（`.codex/skills/repomind-*`）
- 创建 `.claude/rules/repomind.md`（Claude Code 自动加载）+ 更新 `AGENTS.md`（Codex 加载）
- 配置 git hook（提交前自动增量更新图谱）

## 使用

安装后无需手动操作，AI 编码助手自动执行：

- **编码前** — `repomind-query`：读取知识库索引 → 匹配业务模块 → 定位关键代码
- **编码后** — `repomind-summary`：分析变更影响 → 更新模块文档和索引

首次在项目中安装后，知识库为空，需要初始化：

> 在 Claude Code / Codex 中执行 `/repomind-init`

AI 会自动完成：运行 graphify 构建代码图谱 → 归纳业务模块 → 创建模块文档和索引。

## 命令

```bash
repomind install      # 初始化知识库
repomind uninstall    # 移除
repomind update       # 更新到最新版本
```
