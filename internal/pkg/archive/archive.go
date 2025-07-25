// Package archive provides functionality for creating and compressing TAR archives.
package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	gitDirName = ".git"
)

// validatePath ensures the path is safe and within the expected directory
func validatePath(path, baseDir string) (string, error) {
	// Get absolute paths
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute base directory: %w", err)
	}

	// Check if the path is within the base directory
	if !strings.HasPrefix(absPath, absBaseDir) {
		return "", fmt.Errorf("path %s is outside base directory %s", absPath, absBaseDir)
	}

	return absPath, nil
}

// shouldSkipFile determines if a file should be skipped during archiving
func shouldSkipFile(info os.FileInfo, relPath string) bool {
	// Skip the .git directory
	if info.IsDir() && info.Name() == gitDirName {
		return true
	}
	// Skip the root directory itself
	if relPath == "." {
		return true
	}
	return false
}

// createTarHeader creates a tar header for a file
func createTarHeader(info os.FileInfo, relPath string) (*tar.Header, error) {
	header, err := tar.FileInfoHeader(info, relPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create tar header: %w", err)
	}
	// Set the name to the relative path
	header.Name = relPath
	return header, nil
}

// addFileToTar adds a file to the tar archive
func addFileToTar(tarWriter *tar.Writer, path, sourceDir string) error {
	// Validate path to prevent file inclusion vulnerabilities
	safePath, err := validatePath(path, sourceDir)
	if err != nil {
		return fmt.Errorf("invalid path %s: %w", path, err)
	}

	//nolint: gosec
	file, err := os.Open(safePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log error but don't fail the function
			fmt.Printf("Warning: failed to close file %s: %v\n", path, closeErr)
		}
	}()

	if _, err := io.Copy(tarWriter, file); err != nil {
		return fmt.Errorf("failed to copy file %s to tar: %w", path, err)
	}
	return nil
}

// walkAndArchive walks through the directory and adds files to the tar archive
func walkAndArchive(sourceDir string, tarWriter *tar.Writer) error {
	if err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk path %s: %w", path, err)
		}

		// Calculate the relative path for the TAR archive
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Check if file should be skipped
		if shouldSkipFile(info, relPath) {
			if info.IsDir() && info.Name() == gitDirName {
				return filepath.SkipDir
			}
			return nil
		}

		// Create the TAR header
		header, err := createTarHeader(info, relPath)
		if err != nil {
			return err
		}

		// Write the header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		// If it's a regular file, copy its contents
		if !info.IsDir() {
			if err := addFileToTar(tarWriter, path, sourceDir); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}
	return nil
}

// CreateGzippedTarBase64 creates a TAR archive of the given directory, compresses it with gzip,
// and returns the Base64 encoded representation.
func CreateGzippedTarBase64(sourceDir string) (string, error) {
	// Create a buffer to hold the TAR archive
	var buf bytes.Buffer

	// Create a gzip writer
	gzipWriter := gzip.NewWriter(&buf)
	defer func() {
		if err := gzipWriter.Close(); err != nil {
			// Log error but don't fail the function
			fmt.Printf("Warning: failed to close gzip writer: %v\n", err)
		}
	}()

	// Create a TAR writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		if err := tarWriter.Close(); err != nil {
			// Log error but don't fail the function
			fmt.Printf("Warning: failed to close tar writer: %v\n", err)
		}
	}()

	// Walk through the source directory and archive files
	if err := walkAndArchive(sourceDir, tarWriter); err != nil {
		return "", fmt.Errorf("failed to walk directory: %w", err)
	}

	// Close the writers to ensure all data is written
	if err := tarWriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Encode to Base64
	base64Data := base64.StdEncoding.EncodeToString(buf.Bytes())
	return base64Data, nil
}

// CreateTempDirAndCopy creates a temporary directory and copies all contents
// from the current working directory to it, excluding the .git directory.
func CreateTempDirAndCopy(sourceDir string) (string, error) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "nina-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Copy all contents from source directory to temp directory
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the .git directory
		if info.IsDir() && info.Name() == gitDirName {
			return filepath.SkipDir
		}

		// Calculate the relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create the destination path
		destPath := filepath.Join(tempDir, relPath)

		if info.IsDir() {
			// Create directory
			if err := os.MkdirAll(destPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}
		} else {
			// Create parent directories if they don't exist
			if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
				return fmt.Errorf("failed to create parent directories for %s: %w", destPath, err)
			}

			// Copy file
			if err := copyFile(path, destPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to copy file %s to %s: %w", path, destPath, err)
			}
		}

		return nil
	})
	if err != nil {
		// Clean up temp directory on error
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			// Log error but don't fail the function
			fmt.Printf("Warning: failed to remove temp directory %s: %v\n", tempDir, removeErr)
		}
		return "", fmt.Errorf("failed to copy directory contents: %w", err)
	}

	return tempDir, nil
}

// copyFile copies a single file from src to dst with the specified mode
func copyFile(src, dst string, mode os.FileMode) error {
	// For file copying, we trust the paths since they come from filepath.Walk
	// which already provides safe paths relative to the source directory
	//nolint: gosec
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if closeErr := sourceFile.Close(); closeErr != nil {
			// Log error but don't fail the function
			fmt.Printf("Warning: failed to close source file %s: %v\n", src, closeErr)
		}
	}()

	//nolint: gosec
	destFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil {
			// Log error but don't fail the function
			fmt.Printf("Warning: failed to close destination file %s: %v\n", dst, closeErr)
		}
	}()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}
