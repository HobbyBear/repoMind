//go:build !windows

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefreshProjectWithBinaryUsesProjectDirectoryAndPropagatesFailure(t *testing.T) {
	projectRoot := t.TempDir()
	marker := filepath.Join(projectRoot, "sync-invocation.txt")
	script := filepath.Join(t.TempDir(), "repomind-new")
	content := "#!/bin/sh\nprintf '%s\\n%s\\n' \"$PWD\" \"$1\" > \"" + marker + "\"\nexit 7\n"
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := refreshProjectWithBinary(script, projectRoot, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected sync-project failure to be returned")
	}

	data, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatalf("read invocation marker: %v", readErr)
	}
	want := projectRoot + "\nsync-project\n"
	if strings.ReplaceAll(string(data), "\\", "/") != strings.ReplaceAll(want, "\\", "/") {
		t.Fatalf("unexpected sync invocation:\n%s", data)
	}
}
