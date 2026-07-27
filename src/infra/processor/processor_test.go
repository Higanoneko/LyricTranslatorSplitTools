package processor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcessCombineIntegration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lyrictools-combine-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	jpPath := filepath.Join(tmpDir, "song_jp.lrc")
	cnPath := filepath.Join(tmpDir, "song_cn.lrc")

	jpContent := `[ti:Song Title]
[ar:Artist]
[00:01.00]JP Line 1
[00:02.00]JP Line 2
[00:03.00]JP Line 3
`
	cnContent := `[ar:Artist]
[00:01.00]CN Line 1
[00:03.00]CN Line 3
[00:04.00]CN Line 4
`

	if err := os.WriteFile(jpPath, []byte(jpContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cnPath, []byte(cnContent), 0644); err != nil {
		t.Fatal(err)
	}

	result := ProcessCombine([]string{jpPath, cnPath}, CombineConfig{})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Errorf("output file does not exist: %s", result.OutputPath)
	}

	t.Logf("output: %s, groups: %d, warnings: %v", result.OutputPath, len(result.Groups), result.Warnings)
}

func TestProcessCombineWithOutputPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lyrictools-combine-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	jpPath := filepath.Join(tmpDir, "song_jp.lrc")
	cnPath := filepath.Join(tmpDir, "song_cn.lrc")
	outPath := filepath.Join(tmpDir, "output.lrc")

	os.WriteFile(jpPath, []byte("[00:01.00]Line 1\n"), 0644)
	os.WriteFile(cnPath, []byte("[00:01.00]Line A\n"), 0644)

	result := ProcessCombine([]string{jpPath, cnPath}, CombineConfig{
		OutputPath: outPath,
	})

	if result.Error != nil {
		t.Fatal(result.Error)
	}

	foundLine1 := false
	foundLineA := false
	lines, _ := ReadLines(outPath)
	for _, l := range lines {
		if l == "[00:01.00]Line 1" {
			foundLine1 = true
		}
		if l == "[00:01.00]Line A" {
			foundLineA = true
		}
	}
	if !foundLine1 || !foundLineA {
		t.Errorf("output missing expected lines, got: %v", lines)
	}
}

func TestProcessCombineNoTimestamp(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lyrictools-combine-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	jpPath := filepath.Join(tmpDir, "jp.txt")
	cnPath := filepath.Join(tmpDir, "cn.txt")

	os.WriteFile(jpPath, []byte("JP Line 1\nJP Line 2\n"), 0644)
	os.WriteFile(cnPath, []byte("Extra header\nCN Line 1\nCN Line 2\n"), 0644)

	result := ProcessCombine([]string{jpPath, cnPath}, CombineConfig{})

	if result.Error != nil {
		t.Fatal(result.Error)
	}
	if len(result.Groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(result.Groups))
	}
}

func TestProcessCombineNotEnoughFiles(t *testing.T) {
	result := ProcessCombine([]string{"file1.lrc"}, CombineConfig{})

	if result.Error == nil {
		t.Error("expected error for single file")
	}
}
