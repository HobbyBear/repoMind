package kb

import (
	"fmt"
	"sort"
	"strings"
)

type AuditFile struct {
	File      string `json:"file"`
	Name      string `json:"name"`
	Kind      Kind   `json:"kind"`
	SizeBytes int    `json:"size_bytes"`
	Reason    string `json:"reason"`
}

type AuditReport struct {
	Score               int                  `json:"score"`
	Documents           int                  `json:"documents"`
	TotalBytes          int                  `json:"total_bytes"`
	ActiveDocuments     int                  `json:"active_documents"`
	DraftDocuments      int                  `json:"draft_documents"`
	DeprecatedDocuments int                  `json:"deprecated_documents"`
	OversizedFiles      []AuditFile          `json:"oversized_files"`
	MissingOnboarding   []string             `json:"missing_onboarding"`
	IssueCounts         map[string]int       `json:"issue_counts"`
	Validation          ValidationReport     `json:"validation"`
	NewcomerChecks      []AuditNewcomerCheck `json:"newcomer_checks"`
	Recommendations     []string             `json:"recommendations"`
}

type AuditNewcomerCheck struct {
	Query        string   `json:"query"`
	TopFiles     []string `json:"top_files"`
	ProjectFirst bool     `json:"project_first"`
}

func Audit(projectRoot string) (*AuditReport, error) {
	docs, err := scanDocuments(projectRoot)
	if err != nil {
		return nil, err
	}
	report := &AuditReport{
		Documents: len(docs), OversizedFiles: []AuditFile{}, MissingOnboarding: []string{},
		IssueCounts: map[string]int{}, NewcomerChecks: []AuditNewcomerCheck{}, Recommendations: []string{},
	}
	report.Validation = validateDocuments(docs)
	var project *scannedDocument
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
		if doc.Kind == KindProject {
			project = doc
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
	if project == nil {
		report.MissingOnboarding = append(report.MissingOnboarding, "缺少 project.md，陌生读者没有系统入口")
	} else {
		if project.Status != "active" {
			report.MissingOnboarding = append(report.MissingOnboarding, "project.md 仍是 draft")
		}
		for _, heading := range []string{"这是一个什么系统", "主要能力", "业务边界", "术语速查", "推荐阅读顺序"} {
			if strings.TrimSpace(extractSection(project.Body, heading)) == "" {
				report.MissingOnboarding = append(report.MissingOnboarding, "project.md 缺少“"+heading+"”")
			}
		}
	}
	queries := []string{"这个系统是做什么的，新人应该先看什么", "核心业务流程和模块怎么协作"}
	for _, query := range queries {
		result, searchErr := Search(projectRoot, SearchOptions{Query: query, Limit: 3})
		if searchErr != nil {
			return nil, searchErr
		}
		check := AuditNewcomerCheck{Query: query, TopFiles: []string{}}
		for _, item := range result.Results {
			check.TopFiles = append(check.TopFiles, item.File)
		}
		check.ProjectFirst = len(check.TopFiles) > 0 && check.TopFiles[0] == "project.md"
		report.NewcomerChecks = append(report.NewcomerChecks, check)
		if !check.ProjectFirst {
			report.MissingOnboarding = append(report.MissingOnboarding, fmt.Sprintf("新人问题“%s”未优先命中 project.md", query))
		}
	}
	sort.Slice(report.OversizedFiles, func(i, j int) bool { return report.OversizedFiles[i].SizeBytes > report.OversizedFiles[j].SizeBytes })
	if len(report.OversizedFiles) > 0 {
		report.Recommendations = append(report.Recommendations, "优先处理最大的文档：删除时间线、一次性 ID、重复结论，再按业务主题拆分")
	}
	if report.IssueCounts["description_too_long"] > 0 || report.IssueCounts["too_many_keywords"] > 0 {
		report.Recommendations = append(report.Recommendations, "收紧路由元数据：description 只回答何时打开，keywords 保留 3-8 个判别词")
	}
	if len(report.MissingOnboarding) > 0 {
		report.Recommendations = append(report.Recommendations, "先补齐面向新人的项目概览和阅读顺序，再优化深层文档")
	}
	penalty := minInt(report.Validation.Errors*2, 40) + minInt((report.Validation.Warnings+3)/4, 25) + minInt(len(report.MissingOnboarding)*8, 24)
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
