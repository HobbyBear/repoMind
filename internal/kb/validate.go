package kb

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	MaxDescriptionRunes = 120
	MaxKeywords         = 8
	MaxKeywordRunes     = 32
	SoftFileBytes       = 8 * 1024
	HardFileBytes       = 12 * 1024
	SoftSectionBytes    = 2 * 1024
	HardSectionBytes    = 4 * 1024
	SoftLineCount       = 150
)

type ValidationIssue struct {
	Severity        string `json:"severity"`
	Code            string `json:"code"`
	File            string `json:"file"`
	Section         string `json:"section,omitempty"`
	Message         string `json:"message"`
	SuggestedAction string `json:"suggested_action,omitempty"`
}

type ValidationReport struct {
	Valid    bool              `json:"valid"`
	Errors   int               `json:"errors"`
	Warnings int               `json:"warnings"`
	Issues   []ValidationIssue `json:"issues"`
}

func Validate(projectRoot string) (ValidationReport, error) {
	return ValidateFiles(projectRoot, nil)
}

// ValidateFiles validates only the requested .repomind-relative files when
// files is non-empty. This lets incremental writes improve the knowledge base
// without being blocked by unrelated legacy debt.
func ValidateFiles(projectRoot string, files []string) (ValidationReport, error) {
	docs, err := scanDocuments(projectRoot)
	if err != nil {
		return ValidationReport{}, err
	}
	if len(files) > 0 {
		allDocs := docs
		wanted := make(map[string]bool, len(files))
		for _, file := range files {
			file = filepath.ToSlash(strings.TrimSpace(file))
			file = strings.TrimPrefix(file, ".repomind/")
			wanted[file] = true
		}
		filtered := make([]scannedDocument, 0, len(files))
		for _, doc := range docs {
			if wanted[doc.File] {
				filtered = append(filtered, doc)
				delete(wanted, doc.File)
			}
		}
		if len(wanted) > 0 {
			report := validateDocuments(filtered)
			for file := range wanted {
				report.Valid = false
				report.Errors++
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: "error", Code: "file_not_found", File: file,
					Message: "指定的知识文件不存在。", SuggestedAction: "检查 summary 写入路径",
				})
			}
			return report, nil
		}
		report := validateDocuments(filtered)
		selected := make(map[string]bool, len(filtered))
		for _, doc := range filtered {
			selected[doc.File] = true
		}
		for _, doc := range filtered {
			for _, other := range allDocs {
				if selected[other.File] || doc.Kind != other.Kind || !strings.EqualFold(cleanInline(doc.Name), cleanInline(other.Name)) {
					continue
				}
				report.Valid = false
				report.Errors++
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: "error", Code: "duplicate_name", File: doc.File,
					Message: fmt.Sprintf("与 %s 使用了相同的 name。", other.File), SuggestedAction: "合并重复知识或使用更明确的名称",
				})
				break
			}
		}
		return report, nil
	}
	return validateDocuments(docs), nil
}

func validateDocuments(docs []scannedDocument) ValidationReport {
	report := ValidationReport{Valid: true, Issues: make([]ValidationIssue, 0)}
	names := make(map[string]string)
	for _, doc := range docs {
		add := func(severity, code, section, message, action string) {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: severity, Code: code, File: doc.File, Section: section,
				Message: message, SuggestedAction: action,
			})
			if severity == "error" {
				report.Errors++
				report.Valid = false
			} else {
				report.Warnings++
			}
		}

		if !doc.hasFrontMatter {
			add("error", "missing_frontmatter", "", "缺少 YAML frontmatter。", "运行 repomind kb-build 自动补齐 name/description/keywords")
		}
		if doc.Status != "active" && doc.Status != "draft" && doc.Status != "deprecated" {
			add("error", "invalid_status", "", fmt.Sprintf("status %q 无效。", doc.Status), "使用 active、draft 或 deprecated")
		}
		if cleanInline(doc.Name) == "" {
			add("error", "missing_name", "", "缺少可用于路由的 name。", "补充 frontmatter.name")
		}
		if cleanInline(doc.Description) == "" {
			add("error", "missing_description", "", "缺少可用于首轮路由的 description。", "补充 1-2 句话的检索摘要")
		} else if utf8.RuneCountInString(doc.Description) > MaxDescriptionRunes {
			add("warning", "description_too_long", "", fmt.Sprintf("description 超过 %d 个字符。", MaxDescriptionRunes), "压缩为能判断是否需要打开本文的 1-2 句话")
		}
		trimmedDescription := strings.TrimSpace(doc.Description)
		if strings.HasSuffix(trimmedDescription, "…") || strings.HasSuffix(trimmedDescription, "...") || strings.Contains(trimmedDescription, "。。") || strings.ContainsAny(lastRune(trimmedDescription), "，,；;：:") {
			add("warning", "description_incomplete", "", "description 像被机械截断或包含异常结尾。", "重写为语义完整的路由句，不使用固定字符截断")
		}
		if strings.Contains(trimmedDescription, "用于定位关键入口、影响范围和修改注意事项") {
			add("warning", "generic_description", "", "description 使用了无法区分文档的通用套话。", "写清具体业务、触发场景和主要边界")
		}
		if strings.Contains(doc.Description, "待补充") {
			add("warning", "placeholder_description", "", "description 仍是待补充内容，无法稳定路由。", "填写用户会在什么场景查阅本文")
		}
		if strings.Contains(doc.Body, "待补充") {
			severity := "warning"
			if doc.Status == "active" {
				severity = "error"
			}
			add(severity, "placeholder_content", "", "正文仍包含待补充内容。", "保持 draft，或补充完成后再发布为 active")
		}
		if len(doc.Keywords) > MaxKeywords {
			add("warning", "too_many_keywords", "", fmt.Sprintf("关键词有 %d 个，建议最多 %d 个。", len(doc.Keywords), MaxKeywords), "删除泛词和同义重复词，保留高判别词")
		}
		for _, keyword := range doc.Keywords {
			if utf8.RuneCountInString(keyword) > MaxKeywordRunes {
				add("warning", "keyword_too_long", "", fmt.Sprintf("关键词 %q 过长。", keyword), "改成用户实际会搜索的短语")
			}
		}
		if doc.SizeBytes > HardFileBytes {
			add("error", "file_must_split", "", fmt.Sprintf("文件为 %d bytes，超过硬限制 %d bytes。", doc.SizeBytes, HardFileBytes), "按独立业务主题拆分，并在原文保留关联链接")
		} else if doc.SizeBytes > SoftFileBytes {
			add("warning", "file_should_split", "", fmt.Sprintf("文件为 %d bytes，超过建议值 %d bytes。", doc.SizeBytes, SoftFileBytes), "由 summary 生成拆分方案，按独立业务主题拆分")
		}
		if doc.LineCount > SoftLineCount {
			add("warning", "file_too_many_lines", "", fmt.Sprintf("文件有 %d 行，超过可快速通读的建议值 %d 行。", doc.LineCount, SoftLineCount), "保留稳定结论，移动个案和历史记录，或按主题拆分")
		}
		for _, section := range doc.Sections {
			if section.Bytes > HardSectionBytes {
				add("error", "section_must_split", section.Title, fmt.Sprintf("章节为 %d bytes，超过硬限制 %d bytes。", section.Bytes, HardSectionBytes), "删除过程日志和重复结论，或按独立主题拆分")
			} else if section.Bytes > SoftSectionBytes {
				add("warning", "section_should_split", section.Title, fmt.Sprintf("章节为 %d bytes，超过建议值 %d bytes。", section.Bytes, SoftSectionBytes), "拆成更聚焦的小节或独立知识文档")
			}
		}
		for _, section := range doc.Sections {
			if strings.Contains(section.Title, "修订记录") || strings.Contains(section.Title, "变更记录") || strings.Contains(section.Title, "排查时间线") {
				add("warning", "revision_log", section.Title, "当前手册中保留了修订或排查时间线。", "将当前有效结论合并到正文，完整演进交给 Git 历史")
			}
		}

		headings := make(map[string]bool)
		for _, section := range doc.Sections {
			headings[section.Title] = true
		}
		for _, required := range requiredSectionGroups(doc.Kind) {
			found := false
			for _, alias := range required {
				if headings[alias] {
					found = true
					break
				}
			}
			if !found {
				add("warning", "missing_section", strings.Join(required, "/"), "缺少约定章节。", "补充该章节；草稿可以暂时保留此警告")
			}
		}

		nameKey := string(doc.Kind) + ":" + strings.ToLower(cleanInline(doc.Name))
		if previous, ok := names[nameKey]; ok && nameKey != "" {
			add("error", "duplicate_name", "", fmt.Sprintf("与 %s 使用了相同的 name。", previous), "合并重复知识或使用更明确的名称")
		} else {
			names[nameKey] = doc.File
		}
	}
	return report
}

func lastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[len(runes)-1])
}

func requiredSectionGroups(kind Kind) [][]string {
	switch kind {
	case KindProject:
		return [][]string{{"这是一个什么系统"}, {"主要能力"}, {"业务边界"}, {"术语速查", "核心术语"}, {"推荐阅读顺序", "从哪里开始"}}
	case KindConcept:
		return [][]string{{"是什么", "这是什么"}, {"核心规则"}}
	case KindModule:
		return [][]string{{"业务描述", "模块职责"}, {"常见修改场景", "包含能力"}, {"关键代码", "技术入口"}}
	case KindTrouble:
		return [][]string{{"问题", "问题现象", "现象"}, {"排查路径", "排查方法"}, {"验证方式", "结果判断"}}
	default:
		return nil
	}
}
