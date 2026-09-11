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
	CodeRefs    []string
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
	fm := frontMatter{
		Name: options.Name, Description: description,
		Keywords: normalizeKeywords(options.Kind, options.Name, fileName, options.Keywords),
		CodeRefs: normalizeCodeRefs(options.CodeRefs), Status: status,
	}
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

## 适用症状

待补充可重复出现的症状族和不适用边界；不要记录某次事故。

## 首查步骤

1. 待补充第一步。

## 判断分支

| 证据或条件 | 结论 | 下一步 |
|---|---|---|
| 待补充可复现证据 | 待确认 | 待补充下一步 |

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
