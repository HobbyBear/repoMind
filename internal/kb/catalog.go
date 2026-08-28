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

func scanDocuments(projectRoot string) ([]scannedDocument, error) {
	repomindDir := filepath.Join(projectRoot, ".repomind")
	var docs []scannedDocument
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
		description = deriveDescription(kind, name, body, "")
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
	b.WriteString("> 此页由 `repomind kb-build` 生成。具体知识请修改对应 Markdown 文件。\n\n")
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
