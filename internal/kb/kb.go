package kb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"repomind/internal/fsutil"
)

const CurrentFormatVersion = 2

const formatStateFile = ".kb-format.json"

type Kind string

const (
	KindConcept Kind = "concept"
	KindModule  Kind = "module"
	KindTrouble Kind = "trouble"
)

type MetadataEntry struct {
	File        string   `json:"file"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords,omitempty"`
}

type MetadataIndex struct {
	FormatVersion int             `json:"format_version"`
	Concepts      []MetadataEntry `json:"concepts"`
	Modules       []MetadataEntry `json:"modules"`
	Troubles      []MetadataEntry `json:"troubles"`
}

type MigrationResult struct {
	FormatVersion int      `json:"format_version"`
	Migrated      []string `json:"migrated"`
	Removed       []string `json:"removed"`
}

type formatState struct {
	Version int `json:"version"`
}

type frontMatter struct {
	Name        string
	Description string
	Keywords    []string
}

type legacyIndex struct {
	Modules []struct {
		File        string `json:"file"`
		Description string `json:"description"`
	} `json:"modules"`
}

func (k Kind) dirName() string {
	switch k {
	case KindConcept:
		return "concepts"
	case KindModule:
		return "modules"
	case KindTrouble:
		return "troubles"
	default:
		return ""
	}
}

func Migrate(projectRoot string) (*MigrationResult, error) {
	repomindDir := filepath.Join(projectRoot, ".repomind")
	if err := fsutil.EnsureDir(repomindDir); err != nil {
		return nil, err
	}

	result := &MigrationResult{FormatVersion: CurrentFormatVersion}
	legacyDescriptions, err := readLegacyModuleDescriptions(repomindDir)
	if err != nil {
		return nil, err
	}

	for _, kind := range []Kind{KindConcept, KindModule, KindTrouble} {
		dir := filepath.Join(repomindDir, kind.dirName())
		if err := fsutil.EnsureDir(dir); err != nil {
			return nil, err
		}
	}

	if err := ensureLegacyModuleDocs(repomindDir, legacyDescriptions, result); err != nil {
		return nil, err
	}

	for _, kind := range []Kind{KindConcept, KindModule, KindTrouble} {
		if err := normalizeDir(repomindDir, kind, legacyDescriptions, result); err != nil {
			return nil, err
		}
	}

	for _, rel := range []string{
		"index.json",
		filepath.Join("concepts", "README.md"),
		filepath.Join("modules", "README.md"),
		filepath.Join("troubles", "README.md"),
	} {
		if removeIfExists(filepath.Join(repomindDir, rel)) {
			result.Removed = append(result.Removed, filepath.ToSlash(filepath.Join(".repomind", rel)))
		}
	}

	if err := writeFormatState(repomindDir, formatState{Version: CurrentFormatVersion}); err != nil {
		return nil, err
	}

	sort.Strings(result.Migrated)
	sort.Strings(result.Removed)
	return result, nil
}

func BuildMetadata(projectRoot string) (*MetadataIndex, error) {
	if _, err := Migrate(projectRoot); err != nil {
		return nil, err
	}

	repomindDir := filepath.Join(projectRoot, ".repomind")
	index := &MetadataIndex{FormatVersion: CurrentFormatVersion}
	for _, kind := range []Kind{KindConcept, KindModule, KindTrouble} {
		items, err := readMetadataDir(repomindDir, kind)
		if err != nil {
			return nil, err
		}
		switch kind {
		case KindConcept:
			index.Concepts = items
		case KindModule:
			index.Modules = items
		case KindTrouble:
			index.Troubles = items
		}
	}
	return index, nil
}

func readMetadataDir(repomindDir string, kind Kind) ([]MetadataEntry, error) {
	dir := filepath.Join(repomindDir, kind.dirName())
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	items := make([]MetadataEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" || strings.EqualFold(entry.Name(), "README.md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		fm, body, _ := splitFrontMatter(string(data))
		name := fm.Name
		if name == "" {
			name = deriveName(entry.Name(), body)
		}
		description := fm.Description
		if description == "" {
			description = deriveDescription(kind, name, body, "")
		}
		items = append(items, MetadataEntry{
			File:        filepath.ToSlash(filepath.Join(kind.dirName(), entry.Name())),
			Name:        name,
			Description: description,
			Keywords:    normalizeKeywords(kind, name, entry.Name(), fm.Keywords),
		})
	}
	return items, nil
}

func ensureLegacyModuleDocs(repomindDir string, legacyDescriptions map[string]string, result *MigrationResult) error {
	modulesDir := filepath.Join(repomindDir, KindModule.dirName())
	for fileName, description := range legacyDescriptions {
		if filepath.Ext(fileName) != ".md" || !isInformative(description) {
			continue
		}
		path := filepath.Join(modulesDir, fileName)
		if fsutil.Exists(path) {
			continue
		}
		name := deriveName(fileName, "")
		desc := deriveDescription(KindModule, name, "", description)
		body := fmt.Sprintf(`# %s

## 业务描述

%s

## 关键代码

- 待补充

## 常见修改场景

- 待补充

## AI 注意事项

- 待补充
`, name, description)
		if err := fsutil.WriteFile(path, renderDocument(frontMatter{
			Name:        name,
			Description: desc,
			Keywords:    normalizeKeywords(KindModule, name, fileName, nil),
		}, body)); err != nil {
			return err
		}
		result.Migrated = append(result.Migrated, filepath.ToSlash(filepath.Join(".repomind", KindModule.dirName(), fileName)))
	}
	return nil
}

func normalizeDir(repomindDir string, kind Kind, legacyDescriptions map[string]string, result *MigrationResult) error {
	dir := filepath.Join(repomindDir, kind.dirName())
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" || strings.EqualFold(entry.Name(), "README.md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fm, body, hasFrontMatter := splitFrontMatter(string(data))
		name := fm.Name
		if name == "" {
			name = deriveName(entry.Name(), body)
		}
		legacyDescription := ""
		if kind == KindModule {
			legacyDescription = legacyDescriptions[entry.Name()]
		}
		description := fm.Description
		if description == "" {
			description = deriveDescription(kind, name, body, legacyDescription)
		}

		keywords := normalizeKeywords(kind, name, entry.Name(), fm.Keywords)
		if hasFrontMatter && fm.Name == name && fm.Description == description && sameStrings(fm.Keywords, keywords) {
			continue
		}
		if err := fsutil.WriteFile(path, renderDocument(frontMatter{
			Name:        name,
			Description: description,
			Keywords:    keywords,
		}, body)); err != nil {
			return err
		}
		result.Migrated = append(result.Migrated, filepath.ToSlash(filepath.Join(".repomind", kind.dirName(), entry.Name())))
	}
	return nil
}

func renderDocument(fm frontMatter, body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.TrimLeft(body, "\n")
	lines := []string{
		"---",
		"name: " + strconv.Quote(cleanInline(fm.Name)),
		"description: " + strconv.Quote(cleanInline(fm.Description)),
	}
	if len(fm.Keywords) > 0 {
		lines = append(lines, "keywords:")
		for _, keyword := range fm.Keywords {
			lines = append(lines, "- "+strconv.Quote(cleanInline(keyword)))
		}
	}
	lines = append(lines, "---")
	doc := strings.Join(lines, "\n") + "\n"
	if body != "" {
		doc += "\n" + body
		if !strings.HasSuffix(doc, "\n") {
			doc += "\n"
		}
	}
	return doc
}

func splitFrontMatter(content string) (frontMatter, string, bool) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return frontMatter{}, normalized, false
	}
	rest := normalized[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end == -1 {
		return frontMatter{}, normalized, false
	}

	var fm frontMatter
	currentListKey := ""
	for _, rawLine := range strings.Split(rest[:end], "\n") {
		line := strings.TrimSpace(rawLine)
		if currentListKey != "" {
			if strings.HasPrefix(line, "- ") {
				value := decodeScalar(strings.TrimSpace(strings.TrimPrefix(line, "- ")))
				if currentListKey == "keywords" && cleanInline(value) != "" {
					fm.Keywords = append(fm.Keywords, cleanInline(value))
				}
				continue
			}
			currentListKey = ""
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = decodeScalar(value)
		switch key {
		case "name":
			fm.Name = cleanInline(value)
		case "description":
			fm.Description = cleanInline(value)
		case "keywords":
			if value == "" {
				currentListKey = "keywords"
				continue
			}
			for _, keyword := range parseInlineKeywords(value) {
				if cleanInline(keyword) != "" {
					fm.Keywords = append(fm.Keywords, cleanInline(keyword))
				}
			}
		}
	}
	body := rest[end+len("\n---\n"):]
	return fm, body, true
}

func parseInlineKeywords(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		value := decodeScalar(strings.TrimSpace(part))
		if cleanInline(value) != "" {
			keywords = append(keywords, cleanInline(value))
		}
	}
	return keywords
}

func decodeScalar(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) >= 2 && raw[0] == '"' {
		if value, err := strconv.Unquote(raw); err == nil {
			return value
		}
	}
	if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		return strings.ReplaceAll(raw[1:len(raw)-1], "''", "'")
	}
	return raw
}

func deriveName(fileName, body string) string {
	if heading := extractFirstHeading(body); heading != "" {
		return cleanTitle(heading)
	}
	return cleanTitle(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
}

func deriveDescription(kind Kind, name, body, legacyDescription string) string {
	switch kind {
	case KindConcept:
		what := firstNonEmpty(extractSection(body, "是什么"), extractFirstParagraph(body))
		scene := firstNonEmpty(extractSection(body, "用户侧表现"), extractSection(body, "为什么有"), extractSection(body, "易混淆概念"))
		switch {
		case what != "" && scene != "":
			return truncate(fmt.Sprintf("%s。适用场景/边界：%s。", what, scene), 120)
		case what != "":
			return truncate(fmt.Sprintf("%s。用于判断相关业务场景和边界。", what), 120)
		default:
			return fmt.Sprintf("%s 相关业务概念。说明它是什么、在哪些场景会被提及，以及和相邻概念的边界。", name)
		}
	case KindModule:
		desc := firstNonEmpty(cleanLegacyDescription(legacyDescription), extractSection(body, "业务描述"), extractFirstParagraph(body))
		if desc == "" {
			return fmt.Sprintf("%s 相关业务模块。用于定位关键入口、影响范围和修改注意事项。", name)
		}
		if !strings.Contains(desc, "用于") && !strings.Contains(desc, "处理") && !strings.Contains(desc, "负责") {
			desc += "。用于定位关键入口、影响范围和修改注意事项。"
		}
		return truncate(desc, 120)
	case KindTrouble:
		symptom := firstNonEmpty(extractSection(body, "问题"), extractSection(body, "问题现象"), extractSection(body, "现象"))
		root := firstNonEmpty(extractSection(body, "根因"), extractSection(body, "当前排查路径"), extractSection(body, "排查路径"))
		switch {
		case symptom != "" && root != "":
			return truncate(fmt.Sprintf("处理%s时查看。首查方向/常见根因：%s。", symptom, root), 120)
		case symptom != "":
			return truncate(fmt.Sprintf("处理%s时查看。包含排查顺序和常见根因。", symptom), 120)
		default:
			return fmt.Sprintf("处理 %s 相关异常时查看。包含现象、排查顺序和常见根因。", name)
		}
	default:
		return ""
	}
}

func cleanLegacyDescription(description string) string {
	description = cleanInline(description)
	if !isInformative(description) {
		return ""
	}
	return description
}

func isInformative(description string) bool {
	description = cleanInline(description)
	if description == "" {
		return false
	}
	lower := strings.ToLower(description)
	if strings.Contains(lower, "todo") || strings.Contains(description, "待补充业务描述") {
		return false
	}
	return true
}

func cleanInline(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func truncate(s string, maxRunes int) string {
	runes := []rune(cleanInline(s))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}

func extractFirstHeading(body string) string {
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func extractFirstParagraph(body string) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var parts []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(parts) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts = append(parts, stripMarkdown(line))
	}
	return cleanInline(strings.Join(parts, " "))
}

func extractSection(body string, titles ...string) string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	for i, line := range lines {
		title, ok := headingTitle(line)
		if !ok || !matchesTitle(title, titles) {
			continue
		}
		var parts []string
		for _, next := range lines[i+1:] {
			if _, isHeading := headingTitle(next); isHeading {
				break
			}
			next = strings.TrimSpace(next)
			if next == "" {
				if len(parts) > 0 {
					break
				}
				continue
			}
			parts = append(parts, stripMarkdown(next))
		}
		return cleanInline(strings.Join(parts, " "))
	}
	return ""
}

func headingTitle(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	line = strings.TrimLeft(line, "#")
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}
	return line, true
}

func matchesTitle(title string, titles []string) bool {
	title = cleanInline(title)
	for _, want := range titles {
		if title == want {
			return true
		}
	}
	return false
}

func stripMarkdown(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "-*0123456789. ")
	line = strings.ReplaceAll(line, "`", "")
	line = strings.ReplaceAll(line, "**", "")
	line = strings.ReplaceAll(line, "__", "")
	line = strings.ReplaceAll(line, "*", "")
	line = strings.ReplaceAll(line, "_", "")
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "概念：")
	line = strings.TrimPrefix(line, "排查：")
	line = strings.TrimPrefix(line, "故障：")
	return cleanInline(line)
}

func cleanTitle(title string) string {
	title = stripMarkdown(title)
	title = strings.TrimPrefix(title, "概念:")
	title = strings.TrimPrefix(title, "排查:")
	title = strings.TrimPrefix(title, "故障:")
	return cleanInline(title)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if cleaned := cleanInline(value); cleaned != "" {
			return cleaned
		}
	}
	return ""
}

func normalizeKeywords(kind Kind, name, fileName string, existing []string) []string {
	seen := make(map[string]bool)
	keywords := make([]string, 0, len(existing)+2)
	appendKeyword := func(value string) {
		value = cleanInline(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		keywords = append(keywords, value)
	}
	for _, keyword := range existing {
		appendKeyword(keyword)
	}
	if kind == KindModule {
		appendKeyword(name)
		appendKeyword(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
	}
	return keywords
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func removeIfExists(path string) bool {
	if !fsutil.Exists(path) {
		return false
	}
	_ = os.Remove(path)
	return true
}

func readLegacyModuleDescriptions(repomindDir string) (map[string]string, error) {
	path := filepath.Join(repomindDir, "index.json")
	if !fsutil.Exists(path) {
		return map[string]string{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var idx legacyIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return map[string]string{}, nil
	}

	descriptions := make(map[string]string, len(idx.Modules))
	for _, module := range idx.Modules {
		if module.File == "" {
			continue
		}
		descriptions[module.File] = module.Description
	}
	return descriptions, nil
}

func writeFormatState(repomindDir string, state formatState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFile(filepath.Join(repomindDir, formatStateFile), string(data)+"\n")
}
