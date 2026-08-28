package kb

import (
	"sort"
)

type AuditFile struct {
	File      string `json:"file"`
	Name      string `json:"name"`
	Kind      Kind   `json:"kind"`
	SizeBytes int    `json:"size_bytes"`
	Reason    string `json:"reason"`
}

type AuditReport struct {
	Score               int              `json:"score"`
	Documents           int              `json:"documents"`
	TotalBytes          int              `json:"total_bytes"`
	ActiveDocuments     int              `json:"active_documents"`
	DraftDocuments      int              `json:"draft_documents"`
	DeprecatedDocuments int              `json:"deprecated_documents"`
	OversizedFiles      []AuditFile      `json:"oversized_files"`
	IssueCounts         map[string]int   `json:"issue_counts"`
	Validation          ValidationReport `json:"validation"`
	Recommendations     []string         `json:"recommendations"`
}

func Audit(projectRoot string) (*AuditReport, error) {
	docs, err := scanDocuments(projectRoot)
	if err != nil {
		return nil, err
	}
	report := &AuditReport{
		Documents: len(docs), OversizedFiles: []AuditFile{},
		IssueCounts: map[string]int{}, Recommendations: []string{},
	}
	report.Validation = validateDocuments(docs)
	for i := range docs {
		doc := &docs[i]
		report.TotalBytes += doc.SizeBytes
		if doc.Status == "active" {
			report.ActiveDocuments++
		} else if doc.Status == "deprecated" {
			report.DeprecatedDocuments++
		} else {
			report.DraftDocuments++
		}
		if doc.SizeBytes > SoftFileBytes {
			reason := "超过建议大小，应删除重复内容或拆分"
			if doc.SizeBytes > HardFileBytes {
				reason = "超过硬限制，必须精简或拆分"
			}
			report.OversizedFiles = append(report.OversizedFiles, AuditFile{File: doc.File, Name: doc.Name, Kind: doc.Kind, SizeBytes: doc.SizeBytes, Reason: reason})
		}
	}
	for _, issue := range report.Validation.Issues {
		report.IssueCounts[issue.Code]++
	}
	sort.Slice(report.OversizedFiles, func(i, j int) bool { return report.OversizedFiles[i].SizeBytes > report.OversizedFiles[j].SizeBytes })
	if len(report.OversizedFiles) > 0 {
		report.Recommendations = append(report.Recommendations, "优先处理最大的文档：删除时间线、一次性 ID、重复结论，再按业务主题拆分")
	}
	if report.IssueCounts["description_too_long"] > 0 || report.IssueCounts["too_many_keywords"] > 0 {
		report.Recommendations = append(report.Recommendations, "收紧路由元数据：description 只回答何时打开，keywords 保留 3-8 个判别词")
	}
	penalty := minInt(report.Validation.Errors*2, 40) + minInt((report.Validation.Warnings+3)/4, 25)
	report.Score = 100 - penalty
	if report.Score < 0 {
		report.Score = 0
	}
	return report, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
