package graph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"repomind/internal/fsutil"
	"repomind/internal/gitutil"
)

type ModuleCandidate struct {
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
}

type Summary struct {
	Mode             string          `json:"mode"`
	Files            []string        `json:"files"`
	ModuleCandidates []ModuleCandidate `json:"module_candidates"`
	EntryFiles       []string        `json:"entry_files,omitempty"`
	Communities      []CommunityInfo `json:"communities,omitempty"`
}

type CommunityInfo struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Nodes []string `json:"nodes"`
}

var entryPatterns = []string{
	"controller", "service", "handler", "route", "router",
	"job", "worker", "consumer", "resolver", "api", "endpoint",
	"command", "usecase",
}

func GraphScan(repoRoot, graphDir string) (*Summary, error) {
	graphJSON := filepath.Join(repoRoot, "graphify-out", "graph.json")
	if fsutil.Exists(graphJSON) {
		s, err := parseGraphJSON(graphJSON, repoRoot)
		if err == nil {
			return s, nil
		}
	}
	return fallbackScan(repoRoot)
}

func parseGraphJSON(path, repoRoot string) (*Summary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g graphData
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, err
	}

	files, _ := gitutil.ListTrackedFiles(repoRoot)
	if files == nil {
		for _, n := range g.Nodes {
			if n.SourceFile != "" {
				files = append(files, n.SourceFile)
			}
		}
	}

	dirMap := make(map[string]map[string]bool)
	for _, n := range g.Nodes {
		if n.SourceFile == "" {
			continue
		}
		d := topDir(n.SourceFile)
		if d == "" {
			continue
		}
		if dirMap[d] == nil {
			dirMap[d] = make(map[string]bool)
		}
		dirMap[d][n.SourceFile] = true
	}

	var candidates []ModuleCandidate
	for d := range dirMap {
		candidates = append(candidates, ModuleCandidate{Name: d, Paths: []string{d}})
	}

	var entryFiles []string
	for _, f := range files {
		base := strings.ToLower(filepath.Base(f))
		name := strings.TrimSuffix(base, filepath.Ext(base))
		for _, pat := range entryPatterns {
			if strings.Contains(name, pat) {
				entryFiles = append(entryFiles, f)
				break
			}
		}
	}

	var communities []CommunityInfo
	if g.Communities != nil {
		for id, nodes := range g.Communities {
			communities = append(communities, CommunityInfo{ID: id, Nodes: nodes})
		}
	}

	return &Summary{
		Mode:             "graphify",
		Files:            files,
		ModuleCandidates: candidates,
		EntryFiles:       entryFiles,
		Communities:      communities,
	}, nil
}

type graphData struct {
	Nodes       []graphNode         `json:"nodes"`
	Edges       []graphEdge         `json:"edges"`
	Communities map[string][]string `json:"communities"`
}

type graphNode struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	FileType   string `json:"file_type"`
	SourceFile string `json:"source_file"`
}

type graphEdge struct {
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	Relation string  `json:"relation"`
	Weight   float64 `json:"weight"`
}

func fallbackScan(repoRoot string) (*Summary, error) {
	files, err := gitutil.ListTrackedFiles(repoRoot)
	if err != nil {
		return nil, err
	}
	dirs := make(map[string][]string)
	for _, f := range files {
		d := topDir(f)
		if d == "" {
			continue
		}
		dirs[d] = append(dirs[d], f)
	}
	var candidates []ModuleCandidate
	for d := range dirs {
		candidates = append(candidates, ModuleCandidate{Name: d, Paths: []string{d}})
	}
	var entryFiles []string
	for _, f := range files {
		base := strings.ToLower(filepath.Base(f))
		name := strings.TrimSuffix(base, filepath.Ext(base))
		for _, pat := range entryPatterns {
			if strings.Contains(name, pat) {
				entryFiles = append(entryFiles, f)
				break
			}
		}
	}
	return &Summary{
		Mode:             "fallback",
		Files:            files,
		ModuleCandidates: candidates,
		EntryFiles:       entryFiles,
	}, nil
}

func topDir(path string) string {
	path = filepath.ToSlash(path)
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func WriteSummary(graphDir string, s *Summary) error {
	path := filepath.Join(graphDir, "summary.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFile(path, string(data))
}
