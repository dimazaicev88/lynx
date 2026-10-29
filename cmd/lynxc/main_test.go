package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSource(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.lynx")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunCompilesToStdout(t *testing.T) {
	path := writeSource(t, "const a:Number=10;\nvar b=20;\n")
	var stdout, stderr bytes.Buffer

	code := run([]string{path}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	want := "const a = 10;\nlet b = 20;\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunWritesOutputFile(t *testing.T) {
	path := writeSource(t, "const a = 1;\n")
	output := filepath.Join(t.TempDir(), "out.js")
	var stdout, stderr bytes.Buffer

	code := run([]string{path, "-o", output}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "const a = 1;\n" {
		t.Errorf("output file = %q", data)
	}
}

func TestRunCheckOnly(t *testing.T) {
	path := writeSource(t, "const a = 1;\n")
	var stdout, stderr bytes.Buffer

	code := run([]string{"-check", path}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunReportsTypeError(t *testing.T) {
	path := writeSource(t, `const a: Number = "x";`+"\n")
	var stdout, stderr bytes.Buffer

	code := run([]string{path}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "error: cannot assign a value of type String") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "^") {
		t.Errorf("stderr lacks caret: %q", stderr.String())
	}
}

func TestRunReadsStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-"}, strings.NewReader("const a = [1, 2];\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.String() != "const a = [1, 2];\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRunUsageAndVersion(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := run(nil, strings.NewReader(""), &stdout, &stderr); code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
		if !strings.Contains(stderr.String(), "Usage:") {
			t.Errorf("stderr = %q", stderr.String())
		}
	})

	t.Run("version", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"-version"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout.String(), Version) {
			t.Errorf("stdout = %q, want version %q", stdout.String(), Version)
		}
	})

	t.Run("missing input file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"does-not-exist.lynx"}, strings.NewReader(""), &stdout, &stderr); code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})
}
