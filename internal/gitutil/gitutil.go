package gitutil

import (
	"os/exec"
	"strings"
)

func GitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func ListTrackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return nonEmptyLines(string(out)), nil
}

func ChangedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return nonEmptyLines(string(out)), nil
}

func DiffSummary(root string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func nonEmptyLines(s string) []string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			result = append(result, l)
		}
	}
	return result
}
