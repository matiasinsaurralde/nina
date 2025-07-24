package archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateGzippedTarBase64(t *testing.T) {
	// Create a temporary test directory
	testDir, err := os.MkdirTemp("", "test-archive-*")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Create some test files and directories
	testFiles := map[string]string{
		"file1.txt":        "content1",
		"file2.txt":        "content2",
		"subdir/file3.txt": "content3",
		".git/config":      "git content", // This should be excluded
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(testDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	// Create the gzipped tar base64
	base64Data, err := CreateGzippedTarBase64(testDir)
	if err != nil {
		t.Fatalf("CreateGzippedTarBase64 failed: %v", err)
	}

	// Verify the base64 data is not empty
	if base64Data == "" {
		t.Fatal("Base64 data is empty")
	}

	// Decode and verify the content
	decodedData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		t.Fatalf("Failed to decode base64 data: %v", err)
	}

	// Create a gzip reader
	gzipReader, err := gzip.NewReader(strings.NewReader(string(decodedData)))
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gzipReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzipReader)

	// Track found files
	foundFiles := make(map[string]bool)
	expectedFiles := []string{"file1.txt", "file2.txt", "subdir/file3.txt"}

	// Read all entries
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read tar entry: %v", err)
		}

		foundFiles[header.Name] = true

		// Verify file content for regular files
		if header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				t.Fatalf("Failed to read file content for %s: %v", header.Name, err)
			}

			expectedContent, exists := testFiles[header.Name]
			if !exists {
				t.Errorf("Unexpected file found in archive: %s", header.Name)
				continue
			}

			if string(content) != expectedContent {
				t.Errorf("File content mismatch for %s: expected %q, got %q", header.Name, expectedContent, string(content))
			}
		}
	}

	// Verify all expected files are present
	for _, expectedFile := range expectedFiles {
		if !foundFiles[expectedFile] {
			t.Errorf("Expected file not found in archive: %s", expectedFile)
		}
	}

	// Verify .git directory is excluded
	if foundFiles[".git/config"] {
		t.Error(".git directory should be excluded from the archive")
	}
}

func TestCreateTempDirAndCopy(t *testing.T) {
	// Create a temporary source directory
	sourceDir, err := os.MkdirTemp("", "test-source-*")
	if err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}
	defer os.RemoveAll(sourceDir)

	// Create some test files and directories
	testFiles := map[string]string{
		"file1.txt":        "content1",
		"file2.txt":        "content2",
		"subdir/file3.txt": "content3",
		".git/config":      "git content", // This should be excluded
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(sourceDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	// Create temp directory and copy contents
	tempDir, err := CreateTempDirAndCopy(sourceDir)
	if err != nil {
		t.Fatalf("CreateTempDirAndCopy failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Verify all expected files are present in temp directory
	for path, expectedContent := range testFiles {
		// Skip .git files as they should be excluded
		if strings.Contains(path, ".git") {
			continue
		}

		fullPath := filepath.Join(tempDir, path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("Failed to read copied file %s: %v", path, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("File content mismatch for %s: expected %q, got %q", path, expectedContent, string(content))
		}
	}

	// Verify .git directory is not copied
	gitPath := filepath.Join(tempDir, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		t.Error(".git directory should not be copied to temp directory")
	}
}

func TestCreateGzippedTarBase64WithEmptyDir(t *testing.T) {
	// Create an empty temporary directory
	testDir, err := os.MkdirTemp("", "test-empty-*")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Create the gzipped tar base64
	base64Data, err := CreateGzippedTarBase64(testDir)
	if err != nil {
		t.Fatalf("CreateGzippedTarBase64 failed: %v", err)
	}

	// For an empty directory, we should still get a valid base64 string
	// (though the tar might be empty or contain just the directory entry)
	if base64Data == "" {
		t.Fatal("Base64 data should not be empty even for empty directory")
	}

	// Verify we can decode it
	_, err = base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		t.Fatalf("Failed to decode base64 data: %v", err)
	}
}
