package kb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"repomind/internal/fsutil"
)

const (
	generatedDir = ".generated"
	catalogFile  = "catalog.json"
)

type Section struct {
	Title   string `json:"title"`
	Preview string `json:"preview,omitempty"`
	Bytes   int    `json:"bytes"`
}

type Document struct {
	File        string    `json:"file"`
	Kind        Kind      `json:"kind"`
	Status      string    `json:"status"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Keywords    []string  `json:"keywords,omitempty"`
	SizeBytes   int       `json:"size_bytes"`
	LineCount   int       `json:"line_count"`
	Sections    []Section `json:"sections"`
}

type Catalog struct {
	FormatVersion int        `json:"format_version"`
	Project       *Document  `json:"project,omitempty"`
	Documents     []Document `json:"documents"`
}

type BuildResult struct {
	FormatVersion int              `json:"format_version"`
	Catalog       string           `json:"catalog"`
	Overview      string           `json:"overview"`
	Documents     int              `json:"documents"`
	Validation    ValidationReport `json:"validation"`
}

type scannedDocument struct {
	Document
	Body            string
	hasFrontMatter  bool
	sectionContents []string
}

// Build compiles the human-authored Markdown into a compact machine catalog
// and a generated README. Authored document bodies are never moved into the
// generated directory.
func Build(projectRoot string) (*BuildResult, error) {
	if _, err := Migrate(projectRoot); err != nil {
		return nil, err
	}
	if err := ensureProjectDocument(projectRoot); err != nil {
		return nil, err
	}
	if _, err := Normalize(projectRoot); err != nil {
		return nil, err
	}

	docs, err := scanDocuments(projectRoot)
	if err != nil {
		return nil, err
	}
	report := validateDocuments(docs)
	catalog := Catalog{FormatVersion: CurrentFormatVersion, Documents: make([]Document, 0)}
	for _, doc := range docs {
		if doc.Kind == KindProject {
			project := doc.Document
			catalog.Project = &project
			continue
		}
		catalog.Documents = append(catalog.Documents, doc.Document)
	}

	repomindDir := filepath.Join(projectRoot, ".repomind")
	generated := filepath.Join(repomindDir, generatedDir)
	if err := fsutil.EnsureDir(generated); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return nil, err
	}
	catalogPath := filepath.Join(generated, catalogFile)
	if err := fsutil.WriteFile(catalogPath, string(data)+"\n"); err != nil {
		return nil, err
	}
	overviewPath := filepath.Join(repomindDir, "README.md")
	if err := fsutil.WriteFile(overviewPath, renderOverview(catalog)); err != nil {
		return nil, err
	}

	return &BuildResult{
		FormatVersion: CurrentFormatVersion,
		Catalog:       filepath.ToSlash(filepath.Join(".repomind", generatedDir, catalogFile)),
		Overview:      filepath.ToSlash(filepath.Join(".repomind", "README.md")),
		Documents:     len(catalog.Documents),
		Validation:    report,
	}, nil
}

func ensureProjectDocument(projectRoot string) error {
	path := filepath.Join(projectRoot, ".repomind", "project.md")
	if fsutil.Exists(path) {
		return nil
	}
	content := `---
name: "项目概览"
description: "待补充：用一两句话说明这是什么系统、服务哪些用户。"
status: draft
keywords:
- "项目概览"
---

# 项目概览

## 这是一个什么系统

待补充。

## 主要能力

- 待补充。

## 业务边界

- 待补充本系统负责什么、不负责什么。

## 术语速查

- 待补充新人首次阅读会遇到的业务术语和缩写。

## 推荐阅读顺序

1. 待补充核心业务流程。
2. 待补充下一步应阅读的 concept 和 module。

## 常用数据查询入口

- 待补充常用报表、数据表或只读查询入口。
`
	return fsutil.WriteFile(path, content)
}

func scanDocuments(projectRoot string) ([]scannedDocument, error) {
	repomindDir := filepath.Join(projectRoot, ".repomind")
	var docs []scannedDocument
	if doc, err := scanDocument(repomindDir, "project.md", KindProject); err != nil {
		return nil, err
	} else if doc != nil {
		docs = append(docs, *doc)
	}
	for _, kind := range []Kind{KindConcept, KindModule, KindTrouble} {
		dir := filepath.Join(repomindDir, kind.dirName())
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" || strings.EqualFold(entry.Name(), "README.md") {
				continue
			}
			rel := filepath.ToSlash(filepath.Join(kind.dirName(), entry.Name()))
			doc, err := scanDocument(repomindDir, rel, kind)
			if err != nil {
				return nil, err
			}
			if doc != nil {
				docs = append(docs, *doc)
			}
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].File < docs[j].File })
	return docs, nil
}

func scanDocument(repomindDir, rel string, kind Kind) (*scannedDocument, error) {
	data, err := os.ReadFile(filepath.Join(repomindDir, filepath.FromSlash(rel)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	fm, body, hasFrontMatter := splitFrontMatter(string(data))
	name := firstNonEmpty(fm.Name, deriveName(filepath.Base(rel), body))
	description := fm.Description
	if description == "" {
		if kind == KindProject {
			description = firstNonEmpty(extractSection(body, "这是一个什么系统"), extractFirstParagraph(body))
		} else {
			description = deriveDescription(kind, name, body, "")
		}
	}
	sections, contents := parseDocumentSections(body)
	return &scannedDocument{
		Document: Document{
			File:        rel,
			Kind:        kind,
			Status:      normalizeStatus(fm.Status),
			Name:        name,
			Description: cleanInline(description),
			Keywords:    normalizeKeywords(kind, name, filepath.Base(rel), fm.Keywords),
			SizeBytes:   len(data),
			LineCount:   strings.Count(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") + 1,
			Sections:    sections,
		},
		Body:            body,
		hasFrontMatter:  hasFrontMatter,
		sectionContents: contents,
	}, nil
}

func parseDocumentSections(body string) ([]Section, []string) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var sections []Section
	var contents []string
	currentTitle := "正文"
	var current []string
	flush := func() {
		content := strings.TrimSpace(strings.Join(current, "\n"))
		if content == "" && currentTitle == "正文" {
			current = nil
			return
		}
		sections = append(sections, Section{
			Title:   currentTitle,
			Preview: truncate(stripMarkdownBlock(content), 180),
			Bytes:   len([]byte(content)),
		})
		contents = append(contents, content)
		current = nil
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			flush()
			currentTitle = cleanInline(strings.TrimPrefix(trimmed, "## "))
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			continue
		}
		current = append(current, line)
	}
	flush()
	return sections, contents
}

func stripMarkdownBlock(content string) string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		line = stripMarkdown(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, " ")
}

func renderOverview(catalog Catalog) string {
	var b strings.Builder
	b.WriteString("# RepoMind 知识库\n\n")
	b.WriteString("> 此页由 `repomind kb-build` 生成。业务说明请修改 `project.md`，具体知识请修改对应 Markdown 文件。\n\n")
	if catalog.Project != nil && catalog.Project.Status == "active" {
		b.WriteString("## 项目简介\n\n")
		b.WriteString(catalog.Project.Description)
		b.WriteString("\n\n")
	} else {
		b.WriteString("## 项目简介\n\n项目简介尚未发布，请在 `project.md` 中补充后将 `status` 改为 `active`。\n\n")
	}
	counts := map[Kind]int{}
	for _, doc := range catalog.Documents {
		if doc.Status == "active" {
			counts[doc.Kind]++
		}
	}
	b.WriteString("## 快速导航\n\n")
	fmt.Fprintf(&b, "- [业务概念](concepts/)：%d 篇，用于理解业务定义、规则与边界。\n", counts[KindConcept])
	fmt.Fprintf(&b, "- [系统模块](modules/)：%d 篇，用于定位模块职责、能力与技术入口。\n", counts[KindModule])
	fmt.Fprintf(&b, "- [故障排查](troubles/)：%d 篇，用于复用问题现象、排查步骤与数据查询。\n", counts[KindTrouble])
	for _, kind := range []Kind{KindModule, KindConcept, KindTrouble} {
		title := map[Kind]string{KindModule: "模块导航", KindConcept: "业务概念", KindTrouble: "故障排查"}[kind]
		b.WriteString("\n## " + title + "\n\n")
		b.WriteString("| 名称 | 简介 |\n|---|---|\n")
		for _, doc := range catalog.Documents {
			if doc.Kind != kind || doc.Status != "active" {
				continue
			}
			fmt.Fprintf(&b, "| [%s](%s) | %s |\n", escapeTable(doc.Name), doc.File, escapeTable(doc.Description))
		}
	}
	return b.String()
}

func escapeTable(value string) string {
	return strings.ReplaceAll(cleanInline(value), "|", "\\|")
}
