package kb

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type CandidateOptions struct {
	Query             string
	SimilarTo         string
	Kind              Kind
	Limit             int
	IncludeDraft      bool
	IncludeDeprecated bool
}

type CandidateResult struct {
	File          string   `json:"file"`
	Kind          Kind     `json:"kind"`
	Status        string   `json:"status"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Keywords      []string `json:"keywords,omitempty"`
	CodeRefs      []string `json:"code_refs,omitempty"`
	Score         int      `json:"score"`
	MatchedFields []string `json:"matched_fields"`
	Reasons       []string `json:"reasons"`
}

type CandidateResponse struct {
	Mode         string            `json:"mode"`
	Query        string            `json:"query,omitempty"`
	SeedFile     string            `json:"seed_file,omitempty"`
	TotalMatches int               `json:"total_matches"`
	Returned     int               `json:"returned"`
	Candidates   []CandidateResult `json:"candidates"`
}

// FindCandidates performs deterministic, read-only recall. It deliberately
// stops at candidate selection: an agent still decides whether two documents
// should be merged, replaced, kept separate, or ignored.
func FindCandidates(projectRoot string, options CandidateOptions) (*CandidateResponse, error) {
	docs, err := scanDocuments(projectRoot)
	if err != nil {
		return nil, err
	}

	query := cleanInline(options.Query)
	seedFile := normalizeKnowledgePath(options.SimilarTo)
	if query == "" && seedFile == "" {
		return nil, fmt.Errorf("either query or similar-to is required")
	}

	var seed *scannedDocument
	if seedFile != "" {
		for i := range docs {
			if docs[i].File == seedFile {
				seed = &docs[i]
				break
			}
		}
		if seed == nil {
			return nil, fmt.Errorf("knowledge document not found: %s", seedFile)
		}
		if options.Kind == "" {
			options.Kind = seed.Kind
		}
		if query == "" {
			query = strings.Join(append([]string{seed.Name, seed.Description}, seed.Keywords...), " ")
		}
	}

	response := &CandidateResponse{
		Mode: "query", Query: cleanInline(options.Query), SeedFile: seedFile,
		Candidates: make([]CandidateResult, 0),
	}
	if seed != nil {
		response.Mode = "similar"
	}

	terms := candidateTerms(query)
	for i := range docs {
		doc := &docs[i]
		if seed != nil && doc.File == seed.File {
			continue
		}
		if options.Kind != "" && doc.Kind != options.Kind {
			continue
		}
		if !candidateStatusAllowed(doc.Status, options) {
			continue
		}
		result := scoreCandidate(*doc, query, terms, seed)
		if result.Score > 0 {
			response.Candidates = append(response.Candidates, result)
		}
	}

	sort.SliceStable(response.Candidates, func(i, j int) bool {
		if response.Candidates[i].Score == response.Candidates[j].Score {
			return response.Candidates[i].File < response.Candidates[j].File
		}
		return response.Candidates[i].Score > response.Candidates[j].Score
	})
	response.TotalMatches = len(response.Candidates)
	limit := options.Limit
	if limit <= 0 {
		limit = 5
	}
	if len(response.Candidates) > limit {
		response.Candidates = response.Candidates[:limit]
	}
	response.Returned = len(response.Candidates)
	return response, nil
}

func scoreCandidate(doc scannedDocument, rawQuery string, terms []string, seed *scannedDocument) CandidateResult {
	result := CandidateResult{
		File: doc.File, Kind: doc.Kind, Status: doc.Status, Name: doc.Name,
		Description: doc.Description, Keywords: doc.Keywords, CodeRefs: doc.CodeRefs,
		MatchedFields: make([]string, 0), Reasons: make([]string, 0),
	}
	matched := make(map[string]bool)
	addField := func(field, value string, weight, capMatches int) {
		hits := matchingTerms(value, terms)
		if len(hits) == 0 {
			return
		}
		count := len(hits)
		if count > capMatches {
			count = capMatches
		}
		result.Score += count * weight
		matched[field] = true
		result.Reasons = append(result.Reasons, fmt.Sprintf("%s:%s", field, strings.Join(hits[:count], ",")))
	}

	addField("name", doc.Name, 18, 4)
	addField("description", doc.Description, 10, 6)
	addField("keywords", strings.Join(doc.Keywords, " "), 12, 4)
	addField("file", doc.File, 5, 3)
	addField("section", sectionTitles(doc), 7, 4)
	addField("body", doc.Body, 1, 8)

	normalizedQuery := strings.ToLower(cleanInline(rawQuery))
	metadataText := strings.ToLower(cleanInline(strings.Join([]string{doc.Name, doc.Description, strings.Join(doc.Keywords, " ")}, " ")))
	if len([]rune(normalizedQuery)) >= 2 && strings.Contains(metadataText, normalizedQuery) {
		result.Score += 35
		matched["exact_phrase"] = true
		result.Reasons = append(result.Reasons, "exact_phrase")
	}

	for _, codeRef := range doc.CodeRefs {
		leaf := strings.ToLower(codeRefLeaf(codeRef))
		switch {
		case normalizedQuery != "" && strings.Contains(normalizedQuery, strings.ToLower(codeRef)):
			result.Score += 140
			matched["code_refs"] = true
			result.Reasons = append(result.Reasons, "code_ref_exact:"+codeRef)
		case len(leaf) >= 4 && normalizedQuery != "" && strings.Contains(normalizedQuery, leaf):
			result.Score += 80
			matched["code_refs"] = true
			result.Reasons = append(result.Reasons, "code_ref_symbol:"+codeRef)
		}
	}

	if seed != nil {
		exactRefs, symbolRefs := sharedCodeRefs(seed.CodeRefs, doc.CodeRefs)
		if len(exactRefs) > 0 {
			result.Score += minInt(len(exactRefs), 2) * 140
			matched["code_refs"] = true
			result.Reasons = append(result.Reasons, "shared_code_ref:"+strings.Join(exactRefs, ","))
		} else if len(symbolRefs) > 0 {
			result.Score += minInt(len(symbolRefs), 2) * 35
			matched["code_refs"] = true
			result.Reasons = append(result.Reasons, "shared_symbol:"+strings.Join(symbolRefs, ","))
		}
		sharedKeywords := intersectFold(seed.Keywords, doc.Keywords)
		if len(sharedKeywords) > 0 {
			result.Score += minInt(len(sharedKeywords), 3) * 8
			matched["shared_keywords"] = true
			result.Reasons = append(result.Reasons, "shared_keywords:"+strings.Join(sharedKeywords, ","))
		}
	}

	for _, field := range []string{"code_refs", "name", "description", "keywords", "shared_keywords", "file", "section", "body", "exact_phrase"} {
		if matched[field] {
			result.MatchedFields = append(result.MatchedFields, field)
		}
	}
	return result
}

func candidateStatusAllowed(status string, options CandidateOptions) bool {
	switch status {
	case "active":
		return true
	case "draft":
		return options.IncludeDraft
	case "deprecated":
		return options.IncludeDeprecated
	default:
		return false
	}
}

func normalizeKnowledgePath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, ".repomind/")
	return path
}

func candidateTerms(query string) []string {
	segments := strings.FieldsFunc(strings.ToLower(cleanInline(query)), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
	})
	stop := map[string]bool{
		"什么": true, "怎么": true, "如何": true, "是否": true, "这个": true, "一下": true,
		"问题": true, "系统": true, "功能": true, "用户": true, "相关": true, "处理": true,
	}
	seen := make(map[string]bool)
	terms := make([]string, 0)
	appendTerm := func(term string) {
		term = strings.TrimSpace(term)
		if len([]rune(term)) < 2 || stop[term] || seen[term] {
			return
		}
		seen[term] = true
		terms = append(terms, term)
	}
	for _, segment := range segments {
		runes := []rune(segment)
		if containsHanRunes(runes) {
			if len(runes) <= 12 {
				appendTerm(segment)
			}
			for i := 0; i+1 < len(runes); i++ {
				appendTerm(string(runes[i : i+2]))
			}
			continue
		}
		appendTerm(segment)
	}
	return terms
}

func matchingTerms(value string, terms []string) []string {
	value = strings.ToLower(value)
	hits := make([]string, 0)
	for _, term := range terms {
		if strings.Contains(value, term) {
			hits = append(hits, term)
		}
	}
	return hits
}

func containsHanRunes(runes []rune) bool {
	for _, r := range runes {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func sectionTitles(doc scannedDocument) string {
	titles := make([]string, 0, len(doc.Sections))
	for _, section := range doc.Sections {
		titles = append(titles, section.Title)
	}
	return strings.Join(titles, " ")
}

func sharedCodeRefs(left, right []string) ([]string, []string) {
	rightExact := make(map[string]string)
	rightLeaf := make(map[string]string)
	for _, value := range right {
		rightExact[strings.ToLower(value)] = value
		leaf := strings.ToLower(codeRefLeaf(value))
		if len(leaf) >= 4 {
			rightLeaf[leaf] = value
		}
	}
	var exact, symbols []string
	for _, value := range left {
		if _, ok := rightExact[strings.ToLower(value)]; ok {
			exact = append(exact, value)
			continue
		}
		leaf := strings.ToLower(codeRefLeaf(value))
		if _, ok := rightLeaf[leaf]; ok && len(leaf) >= 4 {
			symbols = append(symbols, codeRefLeaf(value))
		}
	}
	return exact, symbols
}

func codeRefLeaf(codeRef string) string {
	codeRef = strings.TrimSpace(codeRef)
	if index := strings.LastIndexAny(codeRef, ".#"); index >= 0 && index+1 < len(codeRef) {
		return strings.Trim(codeRef[index+1:], "()*")
	}
	if index := strings.LastIndex(codeRef, "/"); index >= 0 && index+1 < len(codeRef) {
		return codeRef[index+1:]
	}
	return strings.Trim(codeRef, "()*")
}

func intersectFold(left, right []string) []string {
	set := make(map[string]string)
	for _, value := range right {
		set[strings.ToLower(cleanInline(value))] = value
	}
	result := make([]string, 0)
	for _, value := range left {
		if _, ok := set[strings.ToLower(cleanInline(value))]; ok {
			result = append(result, value)
		}
	}
	return result
}
