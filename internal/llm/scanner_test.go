package llm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arunim2405/terraclaw/internal/llm"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// allExtensions is the pool of extensions used by property tests.
var allExtensions = []string{".tf", ".sh", ".yaml", ".txt", ".json", ".go"}

// matchingExtensions is the set of extensions that RecursiveListGeneratedFiles should include.
var matchingExtensions = map[string]bool{
	".tf":   true,
	".sh":   true,
	".yaml": true,
}

// TestProperty4_RecursiveFileScanningDiscoversAllMatchingFiles verifies that
// for any directory tree containing an arbitrary mix of .tf, .sh, .yaml, and
// non-matching files at arbitrary nesting depths, RecursiveListGeneratedFiles
// returns exactly the set of matching files and no others.
//
// **Validates: Requirements 8.1, 8.2**
func TestProperty4_RecursiveFileScanningDiscoversAllMatchingFiles(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generate a slice of file entries: each entry is (subdirDepth 0-2, fileIndex, extIndex).
	// We use these to construct files in a temp directory.
	type fileEntry struct {
		depth    int
		dirID    int
		fileID   int
		extIndex int
	}

	genFileEntry := gopter.CombineGens(
		gen.IntRange(0, 2),                    // depth
		gen.IntRange(0, 4),                    // dirID
		gen.IntRange(0, 99),                   // fileID
		gen.IntRange(0, len(allExtensions)-1), // extIndex
	).Map(func(vals []interface{}) fileEntry {
		return fileEntry{
			depth:    vals[0].(int),
			dirID:    vals[1].(int),
			fileID:   vals[2].(int),
			extIndex: vals[3].(int),
		}
	})

	genFileEntries := gen.SliceOfN(10, genFileEntry).SuchThat(func(v interface{}) bool {
		s := v.([]fileEntry)
		return len(s) >= 1
	})

	properties.Property("discovers exactly matching files", prop.ForAll(
		func(entries []fileEntry) bool {
			tmpDir, err := os.MkdirTemp("", "scanner-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tmpDir)

			expectedCount := 0
			createdMatching := make(map[string]bool)

			for _, e := range entries {
				ext := allExtensions[e.extIndex]

				// Build subdirectory path based on depth.
				var subDir string
				switch e.depth {
				case 0:
					subDir = ""
				case 1:
					subDir = fmt.Sprintf("dir%d", e.dirID)
				case 2:
					subDir = filepath.Join(fmt.Sprintf("dir%d", e.dirID), fmt.Sprintf("sub%d", e.dirID))
				}

				dir := filepath.Join(tmpDir, subDir)
				if err := os.MkdirAll(dir, 0o750); err != nil {
					return false
				}

				fileName := fmt.Sprintf("file%d%s", e.fileID, ext)
				relPath := filepath.Join(subDir, fileName)
				absPath := filepath.Join(tmpDir, relPath)

				// Skip if we already created this exact file.
				if _, err := os.Stat(absPath); err == nil {
					continue
				}

				if err := os.WriteFile(absPath, []byte("test content"), 0o600); err != nil {
					return false
				}

				if matchingExtensions[ext] {
					if !createdMatching[relPath] {
						createdMatching[relPath] = true
						expectedCount++
					}
				}
			}

			files, err := llm.RecursiveListGeneratedFiles(tmpDir)
			if err != nil {
				return false
			}

			if len(files) != expectedCount {
				return false
			}

			// Verify every returned file has a matching extension.
			for _, f := range files {
				ext := filepath.Ext(f.Name)
				if !matchingExtensions[ext] {
					return false
				}
				// Verify it's in our expected set.
				if !createdMatching[f.Name] {
					return false
				}
			}

			return true
		},
		genFileEntries,
	))

	properties.TestingRun(t)
}

// TestProperty5_ScannedFileNamesUseRelativePaths verifies that for any
// GeneratedFile returned by RecursiveListGeneratedFiles, the Name field
// equals the relative path from the scanned root (not absolute, not basename).
//
// **Validates: Requirements 8.3**
func TestProperty5_ScannedFileNamesUseRelativePaths(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generate random subdirectory depth (1-3 levels) and file names.
	type fileSpec struct {
		depth  int
		dirID  int
		fileID int
	}

	genFileSpec := gopter.CombineGens(
		gen.IntRange(1, 3), // depth (at least 1 to ensure subdirectory)
		gen.IntRange(0, 4), // dirID
		gen.IntRange(0, 9), // fileID
	).Map(func(vals []interface{}) fileSpec {
		return fileSpec{
			depth:  vals[0].(int),
			dirID:  vals[1].(int),
			fileID: vals[2].(int),
		}
	})

	genFileSpecs := gen.SliceOfN(5, genFileSpec).SuchThat(func(v interface{}) bool {
		s := v.([]fileSpec)
		return len(s) >= 1
	})

	properties.Property("Name is relative path, not absolute and not basename", prop.ForAll(
		func(specs []fileSpec) bool {
			tmpDir, err := os.MkdirTemp("", "scanner-test-*")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tmpDir)

			for _, s := range specs {
				// Build nested directory.
				parts := make([]string, s.depth)
				for i := 0; i < s.depth; i++ {
					parts[i] = fmt.Sprintf("d%d_%d", s.dirID, i)
				}
				subDir := filepath.Join(parts...)
				dir := filepath.Join(tmpDir, subDir)
				if err := os.MkdirAll(dir, 0o750); err != nil {
					return false
				}

				fileName := fmt.Sprintf("file%d.tf", s.fileID)
				absPath := filepath.Join(dir, fileName)

				if err := os.WriteFile(absPath, []byte("test content"), 0o600); err != nil {
					return false
				}
			}

			files, err := llm.RecursiveListGeneratedFiles(tmpDir)
			if err != nil {
				return false
			}

			for _, f := range files {
				// Name must NOT start with "/"
				if strings.HasPrefix(f.Name, "/") {
					return false
				}
				// Name must NOT contain the tmpDir prefix
				if strings.Contains(f.Name, tmpDir) {
					return false
				}
				// Name must contain a path separator (since depth >= 1)
				if !strings.Contains(f.Name, string(filepath.Separator)) {
					return false
				}
				// Name must NOT be just the basename
				base := filepath.Base(f.Name)
				if f.Name == base {
					return false
				}
			}

			return true
		},
		genFileSpecs,
	))

	properties.TestingRun(t)
}

// TestRecursiveListGeneratedFiles_IgnoresNonMatchingExtensions verifies that
// RecursiveListGeneratedFiles only returns .tf, .sh, and .yaml files and
// excludes .txt, .json, .go files.
func TestRecursiveListGeneratedFiles_IgnoresNonMatchingExtensions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create matching files.
	matchFiles := []string{"main.tf", "import.sh", "blueprint.yaml"}
	for _, name := range matchFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("test content"), 0o600); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	// Create non-matching files.
	noMatchFiles := []string{"notes.txt", "data.json", "helper.go"}
	for _, name := range noMatchFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("test content"), 0o600); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	files, err := llm.RecursiveListGeneratedFiles(tmpDir)
	if err != nil {
		t.Fatalf("RecursiveListGeneratedFiles failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}

	// Build a set of returned names.
	nameSet := make(map[string]bool)
	for _, f := range files {
		nameSet[f.Name] = true
	}

	// Verify matching files are present.
	for _, name := range matchFiles {
		if !nameSet[name] {
			t.Errorf("expected %s in results, but not found", name)
		}
	}

	// Verify non-matching files are absent.
	for _, name := range noMatchFiles {
		if nameSet[name] {
			t.Errorf("did not expect %s in results, but found it", name)
		}
	}
}
