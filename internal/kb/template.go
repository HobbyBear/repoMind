package kb

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"repomind/internal/fsutil"
)

type CreateOptions struct {
	Kind        Kind
	Name        string
	Description string
	Keywords    []string
	File        string
	Status      string
}

type CreateResult struct {
	File string `json:"file"`
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}

func Create(projectRoot string, options CreateOptions) (*CreateResult, error) {
	if options.Kind != KindConcept && options.Kind != KindModule && options.Kind != KindTrouble {
		return nil, fmt.Errorf("unsupported kind %q: use concept, module, or trouble", options.Kind)
	}
	options.Name = cleanInline(options.Name)
	if options.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	fileName := strings.TrimSpace(options.File)
	if fileName == "" {
		fileName = slugify(options.Name) + ".md"
	}
	if filepath.Base(fileName) != fileName || filepath.Ext(fileName) != ".md" {
		return nil, fmt.Errorf("file must be a .md filename without directories")
	}
	rel := filepath.ToSlash(filepath.Join(options.Kind.dirName(), fileName))
	path := filepath.Join(projectRoot, ".repomind", filepath.FromSlash(rel))
	if fsutil.Exists(path) {
		return nil, fmt.Errorf("knowledge document already exists: %s", rel)
	}
	description := cleanInline(options.Description)
	if description == "" {
		description = "待补充：用一两句话说明什么时候应该查阅这篇知识。"
	}
	status := strings.ToLower(cleanInline(options.Status))
	if status == "" {
		status = "draft"
	}
	if status != "draft" && status != "active" {
		return nil, fmt.Errorf("unsupported status %q: use draft or active", options.Status)
	}
	fm := frontMatter{Name: options.Name, Description: description, Keywords: normalizeKeywords(options.Kind, options.Name, fileName, options.Keywords), Status: status}
	if err := fsutil.WriteFile(path, renderDocument(fm, templateBody(options.Kind, options.Name))); err != nil {
		return nil, err
	}
	return &CreateResult{File: filepath.ToSlash(filepath.Join(".repomind", rel)), Kind: options.Kind, Name: options.Name}, nil
}

func templateBody(kind Kind, name string) string {
	switch kind {
	case KindConcept:
		return fmt.Sprintf(`# %s

## 这是什么

待补充。

## 核心规则

- 待补充。

## 适用场景与边界

- 待补充。

## 关联知识

- 待补充。
`, name)
	case KindModule:
		return fmt.Sprintf(`# %s

## 模块职责

待补充。

## 包含能力

- 待补充。

## 技术入口

- 待补充入口文件或函数；产品、运营可留空，由工程或 AI 补充。

## 关键约束

- 待补充。

## 关联知识

- 待补充。
`, name)
	case KindTrouble:
		return fmt.Sprintf(`# %s

## 问题现象

待补充用户或监控看到的现象。

## 排查方法

1. 待补充第一步。

## 数据查询

- 待补充只读 SQL、报表或查询入口；不要写真实用户数据和密钥。

## 结果判断

- 待补充不同结果分别说明什么。

## 根因与处理

- 待确认。

## 关联知识

- 待补充。
`, name)
	default:
		return ""
	}
}

func slugify(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "knowledge"
	}
	return result
}
