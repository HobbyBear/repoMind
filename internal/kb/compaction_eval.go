package kb

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const (
	retainedUnitCoverage    = 0.60
	duplicateUnitSimilarity = 0.82
	maxPairwiseUnits        = 2000
)

type CompactionSnapshot struct {
	Documents          int     `json:"documents"`
	TotalBytes         int     `json:"total_bytes"`
	KnowledgeUnits     int     `json:"knowledge_units"`
	DuplicateUnits     int     `json:"duplicate_units"`
	DuplicateRatio     float64 `json:"duplicate_ratio"`
	CodeRefs           int     `json:"code_refs"`
	ValidationErrors   int     `json:"validation_errors"`
	ValidationWarnings int     `json:"validation_warnings"`
	OversizedFiles     int     `json:"oversized_files"`
	QualityScore       int     `json:"quality_score"`
}

type CompactionComparison struct {
	Before                  CompactionSnapshot `json:"before"`
	After                   CompactionSnapshot `json:"after"`
	QualityScoreDelta       int                `json:"quality_score_delta"`
	DocumentReduction       int                `json:"document_reduction"`
	DocumentReductionRatio  float64            `json:"document_reduction_ratio"`
	ByteReduction           int                `json:"byte_reduction"`
	ByteReductionRatio      float64            `json:"byte_reduction_ratio"`
	DuplicateRatioReduction float64            `json:"duplicate_ratio_reduction"`
	LexicalUnitsBefore      int                `json:"lexical_units_before"`
	LexicalUnitsRetained    int                `json:"lexical_units_retained"`
	LexicalUnitRetention    float64            `json:"lexical_unit_retention"`
	EvidenceAnchorsBefore   int                `json:"evidence_anchors_before"`
	EvidenceAnchorsRetained int                `json:"evidence_anchors_retained"`
	EvidenceAnchorRetention float64            `json:"evidence_anchor_retention"`
	CodeRefsBefore          int                `json:"code_refs_before"`
	CodeRefsRetained        int                `json:"code_refs_retained"`
	CodeRefRetention        float64            `json:"code_ref_retention"`
	PotentiallyLostUnits    []string           `json:"potentially_lost_units,omitempty"`
	PotentiallyLostAnchors  []string           `json:"potentially_lost_anchors,omitempty"`
	Risks                   []string           `json:"risks"`
}

type knowledgeUnit struct {
	Text     string
	Shingles map[string]struct{}
}

// CompareCompaction evaluates an original and compacted project using the
// same deterministic rubric. Similarity metrics are review aids, not proof of
// semantic equivalence; potentially lost units are returned for human review.
func CompareCompaction(beforeRoot, afterRoot string) (*CompactionComparison, error) {
	beforeDocs, err := scanDocuments(beforeRoot)
	if err != nil {
		return nil, err
	}
	afterDocs, err := scanDocuments(afterRoot)
	if err != nil {
		return nil, err
	}

	beforeUnits := durableKnowledgeUnits(beforeDocs)
	afterUnits := durableKnowledgeUnits(afterDocs)
	beforeSnapshot := buildCompactionSnapshot(beforeDocs, beforeUnits)
	afterSnapshot := buildCompactionSnapshot(afterDocs, afterUnits)
	retained, lost := retainedUnits(beforeUnits, afterUnits)
	beforeRefs := uniqueCodeRefs(beforeDocs)
	afterRefs := uniqueCodeRefs(afterDocs)
	retainedRefs := intersectRefCount(beforeRefs, afterRefs)
	beforeAnchors := evidenceAnchors(beforeDocs)
	afterAnchors := evidenceAnchors(afterDocs)
	retainedAnchors := intersectAnchorCount(beforeAnchors, afterAnchors)

	comparison := &CompactionComparison{
		Before: beforeSnapshot, After: afterSnapshot,
		QualityScoreDelta:       afterSnapshot.QualityScore - beforeSnapshot.QualityScore,
		DocumentReduction:       beforeSnapshot.Documents - afterSnapshot.Documents,
		DocumentReductionRatio:  reductionRatio(beforeSnapshot.Documents, afterSnapshot.Documents),
		ByteReduction:           beforeSnapshot.TotalBytes - afterSnapshot.TotalBytes,
		ByteReductionRatio:      reductionRatio(beforeSnapshot.TotalBytes, afterSnapshot.TotalBytes),
		DuplicateRatioReduction: beforeSnapshot.DuplicateRatio - afterSnapshot.DuplicateRatio,
		LexicalUnitsBefore:      len(beforeUnits),
		LexicalUnitsRetained:    retained,
		LexicalUnitRetention:    fraction(retained, len(beforeUnits)),
		EvidenceAnchorsBefore:   len(beforeAnchors),
		EvidenceAnchorsRetained: retainedAnchors,
		EvidenceAnchorRetention: fraction(retainedAnchors, len(beforeAnchors)),
		CodeRefsBefore:          len(beforeRefs),
		CodeRefsRetained:        retainedRefs,
		CodeRefRetention:        fraction(retainedRefs, len(beforeRefs)),
		PotentiallyLostUnits:    lost,
		PotentiallyLostAnchors:  missingAnchorValues(beforeAnchors, afterAnchors, 30),
		Risks:                   make([]string, 0),
	}
	if len(beforeRefs) == 0 {
		comparison.CodeRefRetention = 1
	}
	if len(beforeAnchors) == 0 {
		comparison.EvidenceAnchorRetention = 1
	}
	if comparison.LexicalUnitRetention < 0.20 {
		comparison.Risks = append(comparison.Risks, "lexical_unit_retention_below_20_percent")
	}
	if len(beforeAnchors) > 0 && comparison.EvidenceAnchorRetention < 0.80 {
		comparison.Risks = append(comparison.Risks, "evidence_anchor_retention_below_80_percent")
	}
	if len(beforeRefs) > 0 && comparison.CodeRefRetention < 1 {
		comparison.Risks = append(comparison.Risks, "code_refs_lost")
	}
	if afterSnapshot.ValidationErrors > 0 {
		comparison.Risks = append(comparison.Risks, "compacted_output_has_validation_errors")
	}
	if comparison.ByteReductionRatio < 0 {
		comparison.Risks = append(comparison.Risks, "compacted_output_is_larger")
	}
	return comparison, nil
}

func buildCompactionSnapshot(docs []scannedDocument, units []knowledgeUnit) CompactionSnapshot {
	report := validateDocuments(docs)
	snapshot := CompactionSnapshot{
		Documents: len(docs), KnowledgeUnits: len(units),
		ValidationErrors: report.Errors, ValidationWarnings: report.Warnings,
	}
	for _, doc := range docs {
		snapshot.TotalBytes += doc.SizeBytes
		snapshot.CodeRefs += len(doc.CodeRefs)
		if doc.SizeBytes > SoftFileBytes {
			snapshot.OversizedFiles++
		}
	}
	snapshot.DuplicateUnits, snapshot.DuplicateRatio = duplicateUnits(units)
	snapshot.QualityScore = corpusQualityScore(docs, snapshot)
	return snapshot
}

func corpusQualityScore(docs []scannedDocument, snapshot CompactionSnapshot) int {
	if len(docs) == 0 {
		return 0
	}
	validation := maxInt(0, 25-snapshot.ValidationErrors*5-snapshot.ValidationWarnings)
	duplication := int(math.Round(20 * (1 - snapshot.DuplicateRatio)))
	size := int(math.Round(15 * (1 - fraction(snapshot.OversizedFiles, len(docs)))))

	descriptions := 0
	anchored := 0
	sectionFound := 0
	sectionTotal := 0
	for _, doc := range docs {
		if cleanInline(doc.Description) != "" && !strings.Contains(doc.Description, "待补充") {
			descriptions++
		}
		if doc.Kind == KindConcept || len(doc.CodeRefs) > 0 {
			anchored++
		}
		headings := make(map[string]bool)
		for _, section := range doc.Sections {
			headings[section.Title] = true
		}
		for _, group := range requiredSectionGroups(doc.Kind) {
			sectionTotal++
			for _, alias := range group {
				if headings[alias] {
					sectionFound++
					break
				}
			}
		}
	}
	routing := int(math.Round(10*fraction(descriptions, len(docs)) + 5*fraction(anchored, len(docs))))
	structure := 25
	if sectionTotal > 0 {
		structure = int(math.Round(25 * fraction(sectionFound, sectionTotal)))
	}
	score := validation + duplication + size + routing + structure
	if score > 100 {
		return 100
	}
	return maxInt(0, score)
}

func durableKnowledgeUnits(docs []scannedDocument) []knowledgeUnit {
	units := make([]knowledgeUnit, 0)
	for _, doc := range docs {
		lines := strings.Split(strings.ReplaceAll(doc.Body, "\r\n", "\n"), "\n")
		section := ""
		inFence := false
		for _, raw := range lines {
			line := strings.TrimSpace(raw)
			if strings.HasPrefix(line, "```") {
				inFence = !inFence
				continue
			}
			if title, ok := headingTitle(line); ok {
				section = title
				continue
			}
			if isTransientSection(section) || line == "" || line == "---" || (!inFence && isMarkdownTableSeparator(line)) {
				continue
			}
			for _, clause := range splitKnowledgeLine(line) {
				normalized := normalizeKnowledgeUnit(clause)
				if len([]rune(normalized)) < 8 {
					continue
				}
				units = append(units, knowledgeUnit{Text: normalized, Shingles: runeShingles(normalized, 2)})
			}
		}
	}
	return units
}

func splitKnowledgeLine(line string) []string {
	return strings.FieldsFunc(line, func(r rune) bool {
		switch r {
		case '|', '。', '；', ';':
			return true
		default:
			return false
		}
	})
}

func isTransientSection(title string) bool {
	for _, marker := range []string{"修订记录", "变更记录", "排查时间线", "当前状态", "实际案例", "已确认案例", "涉及模块", "案例"} {
		if strings.Contains(title, marker) {
			return true
		}
	}
	return false
}

func isMarkdownTableSeparator(line string) bool {
	trimmed := strings.Trim(line, "| ")
	if trimmed == "" {
		return true
	}
	for _, r := range trimmed {
		if r != '-' && r != ':' && r != '|' && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func normalizeKnowledgeUnit(line string) string {
	line = strings.ToLower(stripMarkdown(line))
	line = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			return r
		case unicode.IsSpace(r):
			return ' '
		default:
			return ' '
		}
	}, line)
	return strings.Join(strings.Fields(line), " ")
}

func runeShingles(text string, width int) map[string]struct{} {
	runes := []rune(strings.ReplaceAll(text, " ", ""))
	result := make(map[string]struct{})
	if len(runes) < width {
		if len(runes) > 0 {
			result[string(runes)] = struct{}{}
		}
		return result
	}
	for i := 0; i+width <= len(runes); i++ {
		result[string(runes[i:i+width])] = struct{}{}
	}
	return result
}

func duplicateUnits(units []knowledgeUnit) (int, float64) {
	if len(units) == 0 {
		return 0, 0
	}
	duplicates := 0
	if len(units) > maxPairwiseUnits {
		seen := make(map[string]bool)
		for _, unit := range units {
			if seen[unit.Text] {
				duplicates++
			} else {
				seen[unit.Text] = true
			}
		}
		return duplicates, fraction(duplicates, len(units))
	}
	for i := range units {
		for j := 0; j < i; j++ {
			if jaccard(units[i].Shingles, units[j].Shingles) >= duplicateUnitSimilarity {
				duplicates++
				break
			}
		}
	}
	return duplicates, fraction(duplicates, len(units))
}

func retainedUnits(before, after []knowledgeUnit) (int, []string) {
	retained := 0
	lost := make([]string, 0)
	for _, original := range before {
		best := 0.0
		for _, compacted := range after {
			if similarity := containment(original.Shingles, compacted.Shingles); similarity > best {
				best = similarity
			}
		}
		if best >= retainedUnitCoverage {
			retained++
			continue
		}
		if len(lost) < 20 {
			lost = append(lost, truncate(original.Text, 160))
		}
	}
	sort.Strings(lost)
	return retained, lost
}

// containment measures how much of the original unit survives in a compacted
// unit. Extra context in a merged decision-table cell does not reduce it.
func containment(original, compacted map[string]struct{}) float64 {
	if len(original) == 0 {
		return 1
	}
	intersection := 0
	for item := range original {
		if _, ok := compacted[item]; ok {
			intersection++
		}
	}
	return fraction(intersection, len(original))
}

func jaccard(left, right map[string]struct{}) float64 {
	if len(left) == 0 && len(right) == 0 {
		return 1
	}
	intersection := 0
	for item := range left {
		if _, ok := right[item]; ok {
			intersection++
		}
	}
	union := len(left) + len(right) - intersection
	return fraction(intersection, union)
}

func uniqueCodeRefs(docs []scannedDocument) map[string]struct{} {
	refs := make(map[string]struct{})
	for _, doc := range docs {
		for _, codeRef := range doc.CodeRefs {
			refs[strings.ToLower(cleanInline(codeRef))] = struct{}{}
		}
	}
	return refs
}

var inlineCodePattern = regexp.MustCompile("`([^`]+)`")
var identifierPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_]{3,}`)

func evidenceAnchors(docs []scannedDocument) map[string]struct{} {
	anchors := make(map[string]struct{})
	for _, doc := range docs {
		for _, codeRef := range doc.CodeRefs {
			addEvidenceAnchor(anchors, codeRef)
		}
		section := ""
		for _, raw := range strings.Split(strings.ReplaceAll(doc.Body, "\r\n", "\n"), "\n") {
			if title, ok := headingTitle(raw); ok {
				section = title
				continue
			}
			if isTransientSection(section) {
				continue
			}
			for _, match := range inlineCodePattern.FindAllStringSubmatch(raw, -1) {
				addEvidenceAnchor(anchors, match[1])
			}
			for _, match := range identifierPattern.FindAllString(raw, -1) {
				if strings.ContainsAny(match, "_./") || hasInternalUpper(match) {
					addEvidenceAnchor(anchors, match)
				}
			}
		}
	}
	return anchors
}

func addEvidenceAnchor(anchors map[string]struct{}, value string) {
	value = cleanInline(value)
	if value == "" || len([]rune(value)) > 240 {
		return
	}
	if strings.HasPrefix(value, "/") && !strings.Contains(value, " ") {
		anchors[strings.ToLower(value)] = struct{}{}
	}
	for _, token := range identifierPattern.FindAllString(value, -1) {
		if strings.Contains(token, "_") || hasInternalUpper(token) {
			anchors[strings.ToLower(token)] = struct{}{}
		}
	}
}

func hasInternalUpper(value string) bool {
	if strings.ToUpper(value) == value {
		return false
	}
	for i, r := range value {
		if i > 0 && unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func intersectRefCount(before, after map[string]struct{}) int {
	count := 0
	for ref := range before {
		if _, ok := after[ref]; ok {
			count++
		}
	}
	return count
}

func intersectAnchorCount(before, after map[string]struct{}) int {
	afterCanonical := canonicalAnchorSet(after)
	count := 0
	for anchor := range before {
		if _, ok := afterCanonical[canonicalEvidenceAnchor(anchor)]; ok {
			count++
		}
	}
	return count
}

func missingAnchorValues(before, after map[string]struct{}, limit int) []string {
	afterCanonical := canonicalAnchorSet(after)
	missing := make([]string, 0)
	for anchor := range before {
		if _, ok := afterCanonical[canonicalEvidenceAnchor(anchor)]; !ok {
			missing = append(missing, anchor)
		}
	}
	sort.Strings(missing)
	if len(missing) > limit {
		missing = missing[:limit]
	}
	return missing
}

func canonicalAnchorSet(anchors map[string]struct{}) map[string]struct{} {
	canonical := make(map[string]struct{}, len(anchors))
	for anchor := range anchors {
		canonical[canonicalEvidenceAnchor(anchor)] = struct{}{}
	}
	return canonical
}

func canonicalEvidenceAnchor(anchor string) string {
	return strings.Map(func(r rune) rune {
		if r == '_' || r == '-' {
			return -1
		}
		return unicode.ToLower(r)
	}, anchor)
}

func reductionRatio(before, after int) float64 {
	if before == 0 {
		return 0
	}
	return float64(before-after) / float64(before)
}

func fraction(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
