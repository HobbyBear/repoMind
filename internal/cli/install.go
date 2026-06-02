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
		filepath.Join(repomindDir, "graph"),
		filepath.Join(repomindDir, "bin"),
	}
	for _, d := range dirs {
		if err := fsutil.EnsureDir(d); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// modules/README.md
	if err := fsutil.WriteFile(filepath.Join(repomindDir, "modules", "README.md"), `# RepoMind Modules

此目录存放业务模块文档。

每个文件记录一个业务模块：
- 业务描述
- 关键代码入口
- 常见修改场景
- AI 注意事项

这些文档由 AI skill（Claude Code / Codex）创建和维护。
除非你清楚自己在做什么，否则不要手动编辑。
`); err != nil {
		return err
	}

	// index.json
	if err := fsutil.WriteFile(filepath.Join(repomindDir, "index.json"), `{"modules": []}`+"\n"); err != nil {
		return err
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
	fmt.Println("  index.json")
	fmt.Println("  modules/")
	fmt.Println("  graph/")
	fmt.Println("  bin/repomind-internal")
	fmt.Println()
	fmt.Println(".claude/skills/")
	fmt.Println("  repomind-query/SKILL.md")
	fmt.Println("  repomind-summary/SKILL.md")
	fmt.Println("  repomind-init/SKILL.md")
	fmt.Println()
	fmt.Println(".codex/skills/")
	fmt.Println("  repomind-query/SKILL.md")
	fmt.Println("  repomind-summary/SKILL.md")
	fmt.Println("  repomind-init/SKILL.md")
	fmt.Println()
	fmt.Println("Git:")
	fmt.Println("  .gitattributes — graphify-out/* 冲突时自动取远端")
	fmt.Println("  pre-commit hook — 提交前 graphify --update")
	fmt.Println()
	fmt.Println(".claude/rules/repomind.md — Claude Code 编码前必读知识库")
	fmt.Println("AGENTS.md — Codex 编码前必读知识库")
	fmt.Println()
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

// pre-commit hook: graphify --update before every commit
func ensurePreCommitHook(gitRoot string) error {
	hookPath := filepath.Join(gitRoot, ".git", "hooks", "pre-commit")
	hook := `#!/bin/sh
# RepoMind pre-commit hook — 提交前增量更新图谱
# 纯代码项目只走 AST，不调 LLM，秒级完成
if command -v graphify >/dev/null 2>&1; then
    graphify --update 2>/dev/null || true
fi
`
	// Append to existing hook file if present
	if fsutil.Exists(hookPath) {
		data, err := os.ReadFile(hookPath)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "graphify --update") {
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
!index.json
!modules/**
!bin/repomind-internal
`
	return os.WriteFile(filepath.Join(projectRoot, ".repomind", ".gitignore"), []byte(gitignore), 0644)
}

// ensureAgentInstructions adds RepoMind instructions so every coding agent
// reads the knowledge base before editing business code.
//
// Claude Code: uses .claude/rules/repomind.md (cleanly separated, no conflicts
// with other plugins).  Codex: appends to AGENTS.md (Codex does not have a
// rules directory equivalent).
func ensureAgentInstructions(projectRoot string) error {
	content := `# RepoMind — 代码问答与编码的优先知识库

	## 核心原则
	**任何涉及代码、业务逻辑、项目结构的问题，都必须先查 RepoMind 知识库再回答。**
	回答后如有新发现，自动更新知识库。

	## 自动触发规则

	### 当你问代码/业务问题时
	1. 我**自动调用 `repomind-query`** 查询知识库
	2. 找到相关模块文档和代码位置后回答
	3. 如果有新发现（知识库没记录到的内容），写入 `.repomind/.query-findings.json`
	4. 回答后**自动调用 `repomind-summary`** 更新知识库

	### 编辑/修改代码前
	1. 必须先执行 `repomind-query` skill 查找相关模块
	2. 理解业务上下文后再动手

	### 编码完成后
	1. 必须执行 `repomind-summary` skill 更新知识库
	2. 包括 graphify 增量更新、模块文档更新、index.json 同步

	### 排查 Bug 时
	1. 先执行 `repomind-query` skill 理解相关业务上下文
	2. 再定位问题
`

	// --- Claude Code: .claude/rules/repomind.md ---
	// Always overwrite — this file is fully managed by RepoMind.
	rulesDir := filepath.Join(projectRoot, ".claude", "rules")
	if err := fsutil.EnsureDir(rulesDir); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "repomind.md"), []byte(content), 0644); err != nil {
		return err
	}

	// --- Codex: AGENTS.md (append) ---
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	if fsutil.Exists(agentsPath) {
		data, err := os.ReadFile(agentsPath)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "repomind-query") {
			return nil
		}
	}
	f, err := os.OpenFile(agentsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n\n<!-- repomind-start -->\n\n%s\n\n<!-- repomind-end -->\n", strings.TrimSpace(content))
	return err
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

// ensureGraphifyCLI checks for graphify, installs via pip if missing, and
// deploys graphify skills to both Claude Code and Codex directories.
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
