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
		Short: "Install RepoMind in the current git repository",
		Long:  "Install RepoMind in the current git repository, creating .repomind/ with modules, graph, skills, and internal tools.",
		RunE:  runInstall,
	}
}

func runInstall(cmd *cobra.Command, args []string) error {
	repoRoot, err := gitutil.GitRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}
	repomindDir := filepath.Join(repoRoot, ".repomind")

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
	summary, _ := graph.GraphScan(repoRoot, filepath.Join(repomindDir, "graph"))
	if summary == nil {
		summary = &graph.Summary{Mode: "fallback"}
	}
	if err := graph.WriteSummary(filepath.Join(repomindDir, "graph"), summary); err != nil {
		return err
	}

	// skill files
	if err := skills.InstallSkills(repoRoot); err != nil {
		return fmt.Errorf("failed to install skills: %w", err)
	}

	// repomind-internal binary
	if err := fsutil.CopyExecutable(filepath.Join(repomindDir, "bin", "repomind-internal")); err != nil {
		return fmt.Errorf("failed to copy internal tool: %w", err)
	}

	// git config
	if err := ensureGitAttributes(repoRoot); err != nil {
		return fmt.Errorf("gitattributes: %w", err)
	}
	if err := ensureMergeDriver(); err != nil {
		return fmt.Errorf("merge driver: %w", err)
	}
	if err := ensurePreCommitHook(repoRoot); err != nil {
		return fmt.Errorf("pre-commit hook: %w", err)
	}

	// update CLAUDE.md
	if err := ensureClaudeMd(repoRoot); err != nil {
		return fmt.Errorf("CLAUDE.md: %w", err)
	}

	// auto-stage everything
	stageAll(repoRoot)

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
	fmt.Println("CLAUDE.md — 已添加 RepoMind 指令，编码前必读知识库")
	fmt.Println()
	fmt.Println("已自动 git add 所有 RepoMind 管理的文件。")
	fmt.Println("提交时 hook 会自动更新 AST 图谱。")
	return nil
}

// .gitattributes: graphify-out/* auto-accept remote on conflict
func ensureGitAttributes(repoRoot string) error {
	path := filepath.Join(repoRoot, ".gitattributes")
	line := "graphify-out/* merge=theirs"
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
func ensurePreCommitHook(repoRoot string) error {
	hookPath := filepath.Join(repoRoot, ".git", "hooks", "pre-commit")
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

// stageAll auto-adds RepoMind-managed paths so they're tracked by git
func stageAll(repoRoot string) {
	paths := []string{
		".repomind",
		".claude",
		".codex",
		".gitattributes",
		"graphify-out",
	}
	for _, p := range paths {
		abs := filepath.Join(repoRoot, p)
		if fsutil.Exists(abs) {
			exec.Command("git", "-C", repoRoot, "add", p).Run()
		}
	}
}

// ensureClaudeMd adds RepoMind instructions to CLAUDE.md so Claude Code
// always reads the knowledge base before editing business code.
func ensureClaudeMd(repoRoot string) error {
	section := `

## repomind

编辑业务代码前，先执行 repomind-query skill 查找相关模块。
编码后执行 repomind-summary skill 更新知识库。

务必在理解业务上下文后再动手修改代码，不要跳过知识库查询。
`

	path := filepath.Join(repoRoot, "CLAUDE.md")
	if fsutil.Exists(path) {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "## repomind") {
			return nil
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(section)
	return err
}

func InternalInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "install",
		Short:  "Install RepoMind in the current git repository",
		RunE:   runInstall,
		Hidden: true,
	}
}

func UninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove RepoMind from the current git repository",
		Long:  "Remove all RepoMind files, skills, git hooks, and configuration from the current git repository.",
		RunE:  runUninstall,
	}
}

func runUninstall(cmd *cobra.Command, args []string) error {
	repoRoot, err := gitutil.GitRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}
	repomindDir := filepath.Join(repoRoot, ".repomind")

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

	// skills
	for _, skillsDir := range []string{
		filepath.Join(repoRoot, ".claude", "skills"),
		filepath.Join(repoRoot, ".codex", "skills"),
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

	// graphify-out/
	graphifyDir := filepath.Join(repoRoot, "graphify-out")
	if fsutil.Exists(graphifyDir) {
		if err := os.RemoveAll(graphifyDir); err == nil {
			fmt.Println("Removed graphify-out/")
		}
	}

	// .gitattributes — remove graphify-out/* line
	cleanGitAttributes(repoRoot)

	// .git/hooks/pre-commit — remove graphify block
	cleanPreCommitHook(repoRoot)

	// CLAUDE.md — remove ## repomind section
	cleanClaudeMd(repoRoot)

	// git config — remove merge.theirs.driver
	exec.Command("git", "config", "--unset", "merge.theirs.driver").Run()

	// auto-stage removals
	stageAll(repoRoot)

	fmt.Println()
	fmt.Println("RepoMind uninstalled.")
	return nil
}

func cleanGitAttributes(repoRoot string) {
	path := filepath.Join(repoRoot, ".gitattributes")
	if !fsutil.Exists(path) {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	var kept []string
	for _, l := range lines {
		if strings.Contains(l, "graphify-out/* merge=theirs") {
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

func cleanPreCommitHook(repoRoot string) {
	path := filepath.Join(repoRoot, ".git", "hooks", "pre-commit")
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

func cleanClaudeMd(repoRoot string) {
	path := filepath.Join(repoRoot, "CLAUDE.md")
	if !fsutil.Exists(path) {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	content := string(data)
	// Remove "## repomind" section and following content until next ## or end
	idx := strings.Index(content, "## repomind")
	if idx == -1 {
		return
	}
	// Find end of section: next "## " after the repomind header
	rest := content[idx+len("## repomind"):]
	nextH2 := strings.Index(rest, "\n## ")
	if nextH2 != -1 {
		content = content[:idx] + rest[nextH2:]
	} else {
		content = content[:idx]
	}
	newContent := strings.TrimSpace(content)
	if newContent == "" {
		os.Remove(path)
		fmt.Println("Removed CLAUDE.md (empty after cleanup)")
		return
	}
	os.WriteFile(path, []byte(newContent+"\n"), 0644)
	fmt.Println("Cleaned CLAUDE.md")
}
