package evml

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSVGWritesRequestedOutput(t *testing.T) {
	origReadFile, origWriteFile, origMakeDirAll := readFile, writeFile, makeDirAll
	defer func() {
		readFile, writeFile, makeDirAll = origReadFile, origWriteFile, origMakeDirAll
	}()

	var wrotePath string
	var wroteContent []byte
	readFile = func(name string) ([]byte, error) {
		return []byte("eventmodeling\ntf 01 cmd AddItem\n"), nil
	}
	writeFile = func(name string, data []byte, perm fs.FileMode) error {
		wrotePath = name
		wroteContent = append([]byte(nil), data...)
		return nil
	}
	makeDirAll = func(path string, perm fs.FileMode) error { return nil }

	var stdout, stderr bytes.Buffer
	code := Run([]string{"svg", "model.evml", "-d", "out"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}
	if want := filepath.Join("out", "model.svg"); wrotePath != want {
		t.Fatalf("output path = %q, want %q", wrotePath, want)
	}
	if !strings.Contains(string(wroteContent), "<svg") {
		t.Fatalf("wrote content is not svg: %s", string(wroteContent))
	}
	if !strings.Contains(stdout.String(), "SVG generated successfully") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunSVGRejectsInvalidInput(t *testing.T) {
	origReadFile := readFile
	defer func() { readFile = origReadFile }()
	readFile = func(name string) ([]byte, error) {
		return []byte("eventmodeling\ntf 01 evt Start\ntf 02 cmd Bad ->> 01\n"), nil
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"svg", "model.evml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run() code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "invalid model") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
