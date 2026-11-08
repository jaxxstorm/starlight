package routerinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageVerified(t *testing.T) {
	dir := t.TempDir()
	p := validPreflight()
	path, err := p.StageVerified(dir, strings.NewReader("test artifact"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("staged outside requested directory: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "test artifact" {
		t.Fatalf("staged data %q, error %v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("staged permissions: %v", info.Mode())
	}
}

func TestStageFailuresLeaveNoFiles(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Preflight)
		data string
	}{
		{"unsupported", func(p *Preflight) { p.Supported = nil }, "test artifact"},
		{"checksum mismatch", nil, "wrong"},
		{"budget exceeded", func(p *Preflight) { p.RequiredBytes = uint64(len("test artifact")) }, "test artifact plus more"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			p := validPreflight()
			if tt.edit != nil {
				tt.edit(&p)
			}
			path, err := p.StageVerified(dir, strings.NewReader(tt.data))
			if err == nil || path != "" {
				t.Fatalf("got path %q and error %v", path, err)
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 0 {
				t.Fatalf("files after failure: %v, error %v", files, err)
			}
		})
	}
}

func TestStageAtExactBudget(t *testing.T) {
	p := validPreflight()
	p.RequiredBytes = uint64(len("test artifact"))
	if _, err := p.StageVerified(t.TempDir(), strings.NewReader("test artifact")); err != nil {
		t.Fatal(err)
	}
}

func TestStageReadFailure(t *testing.T) {
	dir := t.TempDir()
	if _, err := validPreflight().StageVerified(dir, failingReader{}); err == nil {
		t.Fatal("read failure accepted")
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("files after failure: %v, error %v", files, err)
	}
}

func TestFailedStagePreservesExistingFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"starlight": "previous binary", "client.conf": "previous config", "enabled": "1"}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := validPreflight().StageVerified(dir, strings.NewReader("invalid upgrade")); err == nil {
		t.Fatal("invalid upgrade was accepted")
	}
	for name, contents := range files {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != contents {
			t.Fatalf("%s changed after failed stage: %q, %v", name, got, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != len(files) {
		t.Fatalf("temporary file remains: %v, %v", entries, err)
	}
}
