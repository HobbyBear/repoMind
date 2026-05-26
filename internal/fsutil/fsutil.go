package fsutil

import (
	"io"
	"os"
	"path/filepath"
)

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func WriteFile(path, content string) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func CopyExecutable(dst string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}
	src, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
