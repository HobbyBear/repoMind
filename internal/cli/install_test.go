package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertManagedBlockReplacesOnlyRepomindSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")

	original := strings.Join([]string{
		"# Existing",
		"",
		"keep this header",
		"",
		"<!-- repomind-start -->",
		"",
		"old repomind text",
		"",
		"<!-- repomind-end -->",
		"",
		"keep this footer",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := upsertManagedBlock(path, "<!-- repomind-start -->", "<!-- repomind-end -->", "new repomind text"); err != nil {
		t.Fatalf("upsertManagedBlock: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(data)

	if !strings.Contains(got, "keep this header") || !strings.Contains(got, "keep this footer") {
		t.Fatalf("unexpected removal of surrounding content:\n%s", got)
	}
	if strings.Contains(got, "old repomind text") {
		t.Fatalf("old managed block still present:\n%s", got)
	}
	if strings.Count(got, "<!-- repomind-start -->") != 1 || strings.Count(got, "<!-- repomind-end -->") != 1 {
		t.Fatalf("managed block markers not normalized:\n%s", got)
	}
	if !strings.Contains(got, "new repomind text") {
		t.Fatalf("new managed block missing:\n%s", got)
	}
}
