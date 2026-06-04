package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"repomind/internal/fsutil"
	"repomind/internal/gitutil"
	"repomind/internal/graph"
	"repomind/internal/kb"
	"repomind/internal/skills"

	"github.com/spf13/cobra"
)

func InstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install RepoMind in the current directory",
		Long:  "Install RepoMind in the current directory, creating .repomind/ with modules, graph, skills, and internal tools.",
		RunE:  runInstall,
	}
}

func runInstall(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine current directory: %w", err)
	}
	gitRoot, err := gitutil.GitRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}
	repomindDir := filepath.Join(projectRoot, ".repomind")

	dirs := []string{
		filepath.Join(repomindDir, "modules"),
		filepath.Join(repomindDir, "concepts"),
		filepath.Join(repomindDir, "troubles"),
		filepath.Join(repomindDir, "graph"),
		filepath.Join(repomindDir, "bin"),
	}
	for _, d := range dirs {
		if err := fsutil.EnsureDir(d); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// graph summary
	summary, _ := graph.GraphScan(projectRoot, filepath.Join(repomindDir, "graph"))
	if summary == nil {
		summary = &graph.Summary{Mode: "fallback"}
	}
	if err := graph.WriteSummary(filepath.Join(repomindDir, "graph"), summary); err != nil {
		return err
	}

	// graphify CLI + skill deployment
	ensureGraphifyCLI()

	// skill files
	if err := skills.InstallSkills(projectRoot); err != nil {
		return fmt.Errorf("failed to install skills: %w", err)
	}

	// .repomind/.gitignore — ensure key files are tracked
	if err := ensureRepomindGitignore(projectRoot); err != nil {
		return fmt.Errorf("repomind gitignore: %w", err)
	}

	// graphify-out/.gitignore — only commit essential outputs
	if err := ensureGraphifyGitignore(projectRoot); err != nil {
		return fmt.Errorf("graphify gitignore: %w", err)
	}

	// repomind-internal binary
	if err := fsutil.CopyExecutable(filepath.Join(repomindDir, "bin", "repomind-internal")); err != nil {
		return fmt.Errorf("failed to copy internal tool: %w", err)
	}

	if _, err := kb.Migrate(projectRoot); err != nil {
		return fmt.Errorf("knowledge base migration: %w", err)
	}

	// git-level config (uses git root, not project root)
	if err := ensureGitAttributes(gitRoot, projectRoot); err != nil {
		return fmt.Errorf("gitattributes: %w", err)
	}
	if err := ensureMergeDriver(); err != nil {
		return fmt.Errorf("merge driver: %w", err)
	}
	if err := ensurePreCommitHook(gitRoot); err != nil {
		return fmt.Errorf("pre-commit hook: %w", err)
	}

	// update agent instruction files so both Claude Code and Codex
	// always read the knowledge base before editing business code.
	if err := ensureAgentInstructions(projectRoot); err != nil {
		return fmt.Errorf("agent instructions: %w", err)
	}

	// auto-stage everything
	stageAll(gitRoot, projectRoot)

	fmt.Println("RepoMind installed.")
	fmt.Println()
	fmt.Println(".repomind/")
	fmt.Println("  .kb-format.json")
	fmt.Println("  concepts/")
	fmt.Println("  modules/")
	fmt.Println("  troubles/")
	fmt.Println("  graph/")
	fmt.Println("  bin/repomind-internal")
	fmt.Println()
	fmt.Println(".claude/skills/")
	fmt.Println("  repomind-query/SKILL.md")
	fmt.Println("  repomind-summary/SKILL.md")
	fmt.Println("  repomind-init/SKILL.md")
	fmt.Println("  repomind-prd/SKILL.md")
	fmt.Println()
	fmt.Println(".codex/skills/")
	fmt.Println("  repomind-query/SKILL.md")
	fmt.Println("  repomind-summary/SKILL.md")
	fmt.Println("  repomind-init/SKILL.md")
	fmt.Println("  repomind-prd/SKILL.md")
	fmt.Println()
	fmt.Println("Git:")
	fmt.Println("  .gitattributes — graphify-out/* 冲突时自动取远端")
	fmt.Println("  pre-commit hook — 提交前 graphify update .")
	fmt.Println()
	fmt.Println(".claude/rules/repomind.md — Claude Code 编码前必读知识库")
	fmt.Println("AGENTS.md — Codex 编码前必读知识库")
	fmt.Println()
	fmt.Println("知识路由已切换为每个 concepts/modules/troubles 文档自己的 name/description 元数据。")
	fmt.Println("已自动 git add 所有 RepoMind 管理的文件。")
	fmt.Println("提交时 hook 会自动更新 AST 图谱。")
	return nil
}

// .gitattributes: graphify-out/* auto-accept remote on conflict.
// gitRoot is the repository top-level; projectRoot is needed to compute
// the relative path to graphify-out/ for the gitattributes pattern.
func ensureGitAttributes(gitRoot, projectRoot string) error {
	path := filepath.Join(gitRoot, ".gitattributes")
	rel, err := filepath.Rel(gitRoot, filepath.Join(projectRoot, "graphify-out"))
	if err != nil {
		rel = "graphify-out"
	}
	line := rel + "/* merge=theirs"
	if fsutil.Exists(path) {
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), line) {
			return nil
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, line)
	return err
}

// merge.theirs.driver: on conflict, keep remote version
func ensureMergeDriver() error {
	return exec.Command("git", "config", "merge.theirs.driver", "cp %B %A").Run()
}

// pre-commit hook: graphify update before every commit
func ensurePreCommitHook(gitRoot string) error {
	hookPath := filepath.Join(gitRoot, ".git", "hooks", "pre-commit")
	hook := `#!/bin/sh
# RepoMind pre-commit hook — 提交前增量更新图谱
# 纯代码项目只走 AST，不调 LLM，秒级完成
if command -v graphify >/dev/null 2>&1; then
    graphify update . 2>/dev/null || true
fi
`
	// Append to existing hook file if present
	if fsutil.Exists(hookPath) {
		data, err := os.ReadFile(hookPath)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "graphify update") {
			return nil
		}
		f, err := os.OpenFile(hookPath, os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString("\n" + hook)
		return err
	}
	return os.WriteFile(hookPath, []byte(hook), 0755)
}

// stageAll auto-adds RepoMind-managed paths so they're tracked by git.
// gitRoot is used for repo-level files (.gitattributes); projectRoot for
// directory-level files (.repomind, .claude, .codex, graphify-out).
func stageAll(gitRoot, projectRoot string) {
	// Project-level paths
	projectPaths := []string{
		".repomind",
		".claude",
		".codex",
		"graphify-out",
		"AGENTS.md",
	}
	for _, p := range projectPaths {
		abs := filepath.Join(projectRoot, p)
		if fsutil.Exists(abs) {
			exec.Command("git", "-C", projectRoot, "add", p).Run()
		}
	}
	// Repo-level paths
	gitPaths := []string{".gitattributes"}
	for _, p := range gitPaths {
		abs := filepath.Join(gitRoot, p)
		if fsutil.Exists(abs) {
			exec.Command("git", "-C", gitRoot, "add", p).Run()
		}
	}
}

// ensureGraphifyGitignore creates graphify-out/.gitignore so that only
// essential outputs are committed. Internal cache files (.graphify_*,
// cost.json, etc.) stay local.
func ensureGraphifyGitignore(projectRoot string) error {
	gitignore := `# Only commit the essential graph outputs.
# Internal cache files are machine-specific and regeneratable.
*
!.gitignore
!graph.json
!GRAPH_REPORT.md
!manifest.json
!graph.html
!.vocab.txt
`
	dir := filepath.Join(projectRoot, "graphify-out")
	if err := fsutil.EnsureDir(dir); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0644)
}

// ensureRepomindGitignore creates .repomind/.gitignore so that key knowledge
// base files are guaranteed to be tracked by git.
func ensureRepomindGitignore(projectRoot string) error {
	gitignore := `# RepoMind 知识库文件 —— 必须被 git 跟踪
!.kb-format.json
!concepts/**
!modules/**
!troubles/**
!bin/repomind-internal
!bin/repomind-internal.exe

# Legacy central index files are obsolete.
index.json
concepts/README.md
modules/README.md
troubles/README.md
`
	return os.WriteFile(filepath.Join(projectRoot, ".repomind", ".gitignore"), []byte(gitignore), 0644)
}

// ensureAgentInstructions adds RepoMind instructions so every coding agent
// reads the knowledge base before editing business code.
//
// Claude Code: uses .claude/rules/repomind.md (cleanly separated, no conflicts
// with other plugins). Codex: replaces only the managed RepoMind block in
// AGENTS.md (Codex does not have a rules directory equivalent).
func ensureAgentInstructions(projectRoot string) error {
	content := repomindInstructionContent()

	// --- Claude Code: .claude/rules/repomind.md ---
	// Always overwrite — this file is fully managed by RepoMind.
	rulesDir := filepath.Join(projectRoot, ".claude", "rules")
	if err := fsutil.EnsureDir(rulesDir); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "repomind.md"), []byte(content), 0644); err != nil {
		return err
	}

	// --- Codex: AGENTS.md (managed block replace) ---
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	return upsertManagedBlock(agentsPath, "<!-- repomind-start -->", "<!-- repomind-end -->", content)
}

func repomindInstructionContent() string {
	raw := `
# RepoMind — 代码问答与编码的优先知识库

## 核心原则

- 任何涉及代码、业务逻辑、项目结构、异常排查，需求分析，方案设计的问题，都必须先查 RepoMind，再回答或改代码。
- RepoMind 查出来的内容不是“参考一下就算了”，而是回答结论、修改决策、排查路径的凭证和上下文依据。
- 命中的 concepts / modules / troubles / graphify 结果，必须真正进入回答或实现判断；不能查完不用，也不能绕开检索结果直接下结论。
- 如果 RepoMind 命中结果不足以支持结论，必须明确说“当前证据不足”，并继续补查代码或图谱。
- RepoMind 当前采用每个 knowledge 文档 frontmatter 里的 {{BT}}name{{BT}} / {{BT}}description{{BT}} 元数据做首轮路由；其中 {{BT}}description{{BT}} 是首要索引摘要，模块文档还要额外维护 {{BT}}keywords{{BT}} 作为辅助定位词，不依赖集中式 {{BT}}index.json{{BT}} 或目录 README。

## repomind-query 触发时机

以下场景必须先触发 {{BT}}repomind-query{{BT}}：

1. 用户询问业务概念、业务规则、项目结构、代码定位、异常现象时。
2. 准备编辑或修改业务代码前。
3. 排查 Bug、分析“为什么没生效 / 为什么不对 / 是不是 Bug”时。
4. 处理历史 PRD 前，如果需要先理解现有业务知识和模块上下文。

## repomind-query 使用要求

1. 先查知识库元数据，再按命中打开 concepts / modules / troubles / graphify；模块路由要同时参考 {{BT}}name{{BT}} / {{BT}}keywords{{BT}} / {{BT}}description{{BT}}。
2. 最终回答必须基于命中的知识组织，而不是把检索结果放在一边。
3. 如果命中了业务卡片，回答里要体现业务定义、边界或预期。
4. 如果命中了模块文档，回答或改动方案里要体现关键入口、影响范围或注意事项。
5. 如果命中了排查记录，回答里要体现历史现象、判断顺序或常见根因。
6. 如果命中内容和当前代码冲突，以当前代码为准，并明确指出冲突。
7. 如果本轮代码定位不是直接通过现有模块文档完成，而是绕过模块文档去查 graphify / source / {{BT}}rg{{BT}} 才定位到实现，那么本轮结束前必须触发 {{BT}}repomind-summary{{BT}}，把缺失的入口信息或模块关键词补回 RepoMind。

## repomind-summary 触发时机

以下场景必须触发 {{BT}}repomind-summary{{BT}}：

1. 代码修改完成后，只要改动影响业务语义、模块边界、关键入口、排查路径或文档元数据。
2. 问答完成后，只要形成了可复用的新业务知识、模块知识或排查经验。
3. 业务讨论、需求分析、PRD 同步后，只要确认了新的概念边界、规则、历史原因或业务意图。
4. 排查结束后，只要形成了可复用的现象、判断路径、根因、验证方式或修订结论。
5. 本轮存在绕过现有模块文档的直接代码查找时，即使最后只补入口或关键词，也必须触发。
6. 本轮识别出某个模块应新增、删除或收紧 {{BT}}keywords{{BT}} 时，也必须触发。

## repomind-summary 使用要求

1. 先做 summary gate，再决定是否落库。
2. 只沉淀代码不容易直接看出的知识，不重复写显式源码细节。
3. summary 时先维护索引元数据，再维护正文；优先检查 {{BT}}description{{BT}} 是否还适合作为首轮路由摘要，模块文档还要同步检查 {{BT}}keywords{{BT}} 是否覆盖最新别称、入口词和常见搜索词。
4. 发现新知识后不要拖到以后；本轮结束前就闭环到 RepoMind。
5. 如果本轮通过直接代码查找才找到答案，至少要把“缺失的模块入口 / 新增关键词 / 应补的常见修改场景”总结回 RepoMind。
`
	return strings.TrimSpace(strings.ReplaceAll(raw, "{{BT}}", "`"))
}

func upsertManagedBlock(path, startMarker, endMarker, content string) error {
	block := fmt.Sprintf("%s\n\n%s\n\n%s\n", startMarker, strings.TrimSpace(content), endMarker)

	existing := ""
	if fsutil.Exists(path) {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		existing = string(data)
	}

	start := strings.Index(existing, startMarker)
	end := strings.Index(existing, endMarker)
	switch {
	case start != -1 && end != -1 && end > start:
		existing = strings.TrimRight(existing[:start]+existing[end+len(endMarker):], "\n")
	case start != -1:
		existing = strings.TrimRight(existing[:start], "\n")
	default:
		existing = strings.TrimRight(existing, "\n")
	}

	final := block
	if strings.TrimSpace(existing) != "" {
		final = existing + "\n\n" + block
	}
	return os.WriteFile(path, []byte(final), 0644)
}

func InternalInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "install",
		Short:  "Install RepoMind in the current directory",
		RunE:   runInstall,
		Hidden: true,
	}
}

func UninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove RepoMind from the current directory",
		Long:  "Remove all RepoMind files, skills, git hooks, and configuration from the current directory.",
		RunE:  runUninstall,
	}
}

func runUninstall(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine current directory: %w", err)
	}
	gitRoot, err := gitutil.GitRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}
	repomindDir := filepath.Join(projectRoot, ".repomind")

	if !fsutil.Exists(repomindDir) {
		fmt.Println("RepoMind is not installed (no .repomind/ directory found).")
		return nil
	}

	fmt.Print("This will remove all RepoMind files, skills, and configuration. Continue? [y/N] ")
	var answer string
	fmt.Scanln(&answer)
	if strings.ToLower(answer) != "y" && strings.ToLower(answer) != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	// .repomind/
	if err := os.RemoveAll(repomindDir); err != nil {
		return fmt.Errorf("failed to remove .repomind/: %w", err)
	}
	fmt.Println("Removed .repomind/")

	// skills (project-level)
	for _, skillsDir := range []string{
		filepath.Join(projectRoot, ".claude", "skills"),
		filepath.Join(projectRoot, ".codex", "skills"),
	} {
		if fsutil.Exists(skillsDir) {
			entries, _ := os.ReadDir(skillsDir)
			for _, e := range entries {
				if e.IsDir() && strings.HasPrefix(e.Name(), "repomind-") {
					p := filepath.Join(skillsDir, e.Name())
					if err := os.RemoveAll(p); err == nil {
						fmt.Printf("Removed %s\n", p)
					}
				}
			}
		}
	}

	// graphify-out/ (project-level)
	graphifyDir := filepath.Join(projectRoot, "graphify-out")
	if fsutil.Exists(graphifyDir) {
		if err := os.RemoveAll(graphifyDir); err == nil {
			fmt.Println("Removed graphify-out/")
		}
	}

	// .gitattributes — remove graphify-out line (repo-level)
	cleanGitAttributes(gitRoot, projectRoot)

	// .claude/rules/repomind.md
	repomindRule := filepath.Join(projectRoot, ".claude", "rules", "repomind.md")
	if fsutil.Exists(repomindRule) {
		if err := os.Remove(repomindRule); err == nil {
			fmt.Println("Removed .claude/rules/repomind.md")
		}
	}

	// CLAUDE.md + AGENTS.md — remove old repomind block (legacy installs)
	cleanRepomindSection(filepath.Join(projectRoot, "CLAUDE.md"))
	cleanRepomindSection(filepath.Join(projectRoot, "AGENTS.md"))

	// .git/hooks/pre-commit — remove graphify block (repo-level)
	cleanPreCommitHook(gitRoot)

	// git config — remove merge.theirs.driver
	exec.Command("git", "config", "--unset", "merge.theirs.driver").Run()

	// auto-stage removals
	stageAll(gitRoot, projectRoot)

	fmt.Println()
	fmt.Println("RepoMind uninstalled.")
	return nil
}

func cleanGitAttributes(gitRoot, projectRoot string) {
	path := filepath.Join(gitRoot, ".gitattributes")
	if !fsutil.Exists(path) {
		return
	}
	rel, err := filepath.Rel(gitRoot, filepath.Join(projectRoot, "graphify-out"))
	if err != nil {
		rel = "graphify-out"
	}
	pattern := rel + "/* merge=theirs"
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	var kept []string
	for _, l := range lines {
		if strings.Contains(l, pattern) {
			continue
		}
		kept = append(kept, l)
	}
	newContent := strings.TrimSpace(strings.Join(kept, "\n"))
	if newContent == "" {
		os.Remove(path)
		fmt.Println("Removed .gitattributes (empty after cleanup)")
		return
	}
	os.WriteFile(path, []byte(newContent+"\n"), 0644)
	fmt.Println("Cleaned .gitattributes")
}

func cleanPreCommitHook(gitRoot string) {
	path := filepath.Join(gitRoot, ".git", "hooks", "pre-commit")
	if !fsutil.Exists(path) {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	content := string(data)
	// Remove the RepoMind block (from "# RepoMind pre-commit hook" to end of "fi")
	lines := strings.Split(content, "\n")
	var cleaned []string
	inBlock := false
	for _, l := range lines {
		if strings.Contains(l, "RepoMind pre-commit hook") {
			inBlock = true
			continue
		}
		if inBlock {
			if strings.TrimSpace(l) == "fi" {
				inBlock = false
			}
			continue
		}
		cleaned = append(cleaned, l)
	}
	newContent := strings.TrimSpace(strings.Join(cleaned, "\n"))
	if newContent == "" || newContent == "#!/bin/sh" {
		os.Remove(path)
		fmt.Println("Removed pre-commit hook (empty after cleanup)")
		return
	}
	os.WriteFile(path, []byte(newContent+"\n"), 0755)
	fmt.Println("Cleaned pre-commit hook")
}

// cleanRepomindSection removes the <!-- repomind-start -->...<!-- repomind-end -->
// block from CLAUDE.md / AGENTS.md. Used to clean up legacy installs.
func cleanRepomindSection(path string) {
	if !fsutil.Exists(path) {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	content := string(data)
	start := strings.Index(content, "<!-- repomind-start -->")
	if start == -1 {
		return
	}
	end := strings.Index(content, "<!-- repomind-end -->")
	if end == -1 {
		return
	}
	end += len("<!-- repomind-end -->")
	cleaned := strings.TrimSpace(content[:start] + content[end:])
	if cleaned == "" {
		os.Remove(path)
		fmt.Printf("Removed %s (empty after cleanup)\n", filepath.Base(path))
		return
	}
	os.WriteFile(path, []byte(cleaned+"\n"), 0644)
	fmt.Printf("Cleaned %s\n", filepath.Base(path))
}

func ensureGraphifyCLI() {
	_, err := exec.LookPath("graphify")
	if err != nil {
		fmt.Println("graphify not found, installing via pip...")
		pip := "pip3"
		if _, e := exec.LookPath("pip3"); e != nil {
			pip = "pip"
		}
		c := exec.Command(pip, "install", "graphifyy")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if e := c.Run(); e != nil {
			fmt.Println("Warning: graphify installation skipped (pip install failed)")
			return
		}
	}

	fmt.Println("Deploying graphify skills...")
	exec.Command("graphify", "install").Run()
	exec.Command("graphify", "install", "--platform", "codex").Run()
	fmt.Println("graphify ready.")
}
