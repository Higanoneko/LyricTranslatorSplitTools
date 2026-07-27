package processor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGroupCombineFiles(t *testing.T) {
	files := []string{
		"/path/song_jp.lrc",
		"/path/song_cn.lrc",
		"/path/song_ro.lrc",
		"/path/other.lrc",
		"/path/standalone.lrc",
	}

	groups := GroupCombineFiles(files)

	songGroup := groups["song"]
	if len(songGroup) != 3 {
		t.Errorf("song group: expected 3 files, got %d: %v", len(songGroup), songGroup)
	}
	if _, ok := groups["other"]; !ok {
		t.Error("expected 'other' group")
	}
	if _, ok := groups["standalone"]; !ok {
		t.Error("expected 'standalone' group")
	}
}

func TestBatchCombine(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lyrictools-batch-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "song_jp.lrc"), []byte("[ti:Song]\n[00:01.00]JP1\n[00:02.00]JP2\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "song_cn.lrc"), []byte("[00:01.00]CN1\n[00:02.00]CN2\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "other_jp.lrc"), []byte("[ti:Other]\n[00:01.00]O1\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "other_cn.lrc"), []byte("[00:01.00]OC1\n"), 0644)

	result := BatchCombine(tmpDir, tmpDir, CombineConfig{})

	if result.TotalGroups != 2 {
		t.Errorf("TotalGroups = %d, want 2 (song + other)", result.TotalGroups)
	}
	if result.SuccessCount != 2 {
		t.Errorf("SuccessCount = %d, want 2", result.SuccessCount)
	}
	if result.FailCount != 0 {
		t.Errorf("FailCount = %d, want 0", result.FailCount)
	}

	outputFile := filepath.Join(tmpDir, "song_Combined.lrc")
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Errorf("expected output file %s does not exist", outputFile)
	}
}

func TestGroupCombineFilesNoMatch(t *testing.T) {
	files := []string{
		"/path/unique_a.lrc",
		"/path/unique_b.lrc",
	}

	groups := GroupCombineFiles(files)

	if len(groups) != 2 {
		t.Errorf("expected 2 groups (both unique), got %d", len(groups))
	}
}
