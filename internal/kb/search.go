package kb

import (
	"sort"
	"strings"
	"unicode"
)

type SearchOptions struct {
	Query             string
	Kind              Kind
	Limit             int
	IncludeDraft      bool
	IncludeDeprecated bool
}

type SearchResult struct {
	File            string   `json:"file"`
	Kind            Kind     `json:"kind"`
	Status          string   `json:"status"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Keywords        []string `json:"keywords,omitempty"`
	Score           int      `json:"score"`
	MatchedFields   []string `json:"matched_fields"`
	MatchedSections []string `json:"matched_sections,omitempty"`
	Snippets        []string `json:"snippets,omitempty"`
}

type SearchResponse struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
}

func Search(projectRoot string, options SearchOptions) (*SearchResponse, error) {
	docs, err := scanDocuments(projectRoot)
	if err != nil {
		return nil, err
	}
	terms := searchTerms(options.Query)
	response := &SearchResponse{Query: options.Query, Results: make([]SearchResult, 0)}
	for _, doc := range docs {
		if options.Kind != "" && doc.Kind != options.Kind {
			continue
		}
		if doc.Status == "deprecated" && !options.IncludeDeprecated {
			continue
		}
		if doc.Status == "draft" && !options.IncludeDraft {
			continue
		}
		if doc.Status != "active" && doc.Status != "draft" && doc.Status != "deprecated" {
			continue
		}
		result := scoreDocument(doc, options.Query, terms)
		if result.Score > 0 {
			response.Results = append(response.Results, result)
		}
	}
	sort.SliceStable(response.Results, func(i, j int) bool {
		if response.Results[i].Score == response.Results[j].Score {
			return response.Results[i].File < response.Results[j].File
		}
		return response.Results[i].Score > response.Results[j].Score
	})
	limit := options.Limit
	if limit <= 0 {
		limit = 5
	}
	if len(response.Results) > limit {
		response.Results = response.Results[:limit]
	}
	return response, nil
}

func scoreDocument(doc scannedDocument, rawQuery string, terms []string) SearchResult {
	result := SearchResult{
		File: doc.File, Kind: doc.Kind, Status: doc.Status, Name: doc.Name, Description: doc.Description, Keywords: doc.Keywords,
	}
	matchedFields := make(map[string]bool)
	if doc.Kind == KindProject && isOnboardingQuery(rawQuery) {
		result.Score += 60
		matchedFields["onboarding"] = true
	}
	addField := func(field, value string, weight, maxMatches int) {
		value = strings.ToLower(value)
		matched := 0
		for _, term := range terms {
			if strings.Contains(value, term) {
				matched++
			}
		}
		if matched > maxMatches {
			matched = maxMatches
		}
		if matched > 0 {
			result.Score += matched * weight
			matchedFields[field] = true
		}
	}
	addField("name", doc.Name, 15, 4)
	addField("file", doc.File, 4, 3)
	addField("description", doc.Description, 7, 6)
	addField("keywords", strings.Join(doc.Keywords, " "), 12, 4)

	type sectionScore struct {
		title   string
		preview string
		score   int
	}
	var sectionScores []sectionScore
	for i, section := range doc.Sections {
		content := ""
		if i < len(doc.sectionContents) {
			content = doc.sectionContents[i]
		}
		score := termMatchScore(section.Title, terms)*5 + termMatchScore(content, terms)
		if score == 0 {
			continue
		}
		matchedFields["body"] = true
		sectionScores = append(sectionScores, sectionScore{title: section.Title, preview: section.Preview, score: score})
	}
	query := strings.ToLower(cleanInline(rawQuery))
	all := strings.ToLower(doc.Name + "\n" + doc.Description + "\n" + strings.Join(doc.Keywords, "\n") + "\n" + doc.Body)
	if len([]rune(query)) >= 2 && strings.Contains(all, query) {
		result.Score += 20
	}
	sort.SliceStable(sectionScores, func(i, j int) bool { return sectionScores[i].score > sectionScores[j].score })
	for i, section := range sectionScores {
		if i >= 3 {
			break
		}
		sectionContribution := section.score
		if sectionContribution > 12 {
			sectionContribution = 12
		}
		result.Score += sectionContribution
		result.MatchedSections = append(result.MatchedSections, section.title)
		if section.preview != "" {
			result.Snippets = append(result.Snippets, section.preview)
		}
	}
	for _, field := range []string{"onboarding", "name", "keywords", "description", "file", "body"} {
		if matchedFields[field] {
			result.MatchedFields = append(result.MatchedFields, field)
		}
	}
	return result
}

func isOnboardingQuery(query string) bool {
	query = strings.ToLower(cleanInline(query))
	for _, phrase := range []string{"新人", "新手", "系统是做什么", "项目是做什么", "先看什么", "从哪里开始", "阅读顺序", "核心业务流程", "模块怎么协作", "系统概览", "项目概览"} {
		if strings.Contains(query, phrase) {
			return true
		}
	}
	return false
}

func termMatchScore(value string, terms []string) int {
	value = strings.ToLower(value)
	score := 0
	for _, term := range terms {
		if strings.Contains(value, term) {
			score++
		}
	}
	return score
}

func searchTerms(query string) []string {
	query = strings.ToLower(cleanInline(query))
	segments := strings.FieldsFunc(query, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
	})
	stop := map[string]bool{
		"什么": true, "怎么": true, "如何": true, "是否": true, "这个": true,
		"一下": true, "问题": true, "系统": true, "功能": true,
	}
	seen := make(map[string]bool)
	var terms []string
	appendTerm := func(term string) {
		term = strings.TrimSpace(term)
		if term == "" || stop[term] || seen[term] {
			return
		}
		seen[term] = true
		terms = append(terms, term)
	}
	for _, segment := range segments {
		runes := []rune(segment)
		if containsHan(runes) {
			if len(runes) <= 8 {
				appendTerm(segment)
			}
			for i := 0; i+1 < len(runes); i++ {
				appendTerm(string(runes[i : i+2]))
			}
			continue
		}
		if len(runes) >= 2 {
			appendTerm(segment)
		}
	}
	if len(terms) == 0 && query != "" {
		appendTerm(query)
	}
	return terms
}

func containsHan(runes []rune) bool {
	for _, r := range runes {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
