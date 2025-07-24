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
)

// CreateGzippedTarBase64 creates a TAR archive of the given directory, compresses it with gzip,
// and returns the Base64 encoded representation.
func CreateGzippedTarBase64(sourceDir string) (string, error) {
	// Create a buffer to hold the TAR archive
	var buf bytes.Buffer

	// Create a gzip writer
	gzipWriter := gzip.NewWriter(&buf)
	defer gzipWriter.Close()

	// Create a TAR writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Walk through the source directory
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Calculate the relative path for the TAR archive
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create the TAR header
		header, err := tar.FileInfoHeader(info, relPath)
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}

		// Set the name to the relative path
		header.Name = relPath

		// Write the header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		// If it's a regular file, copy its contents
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", path, err)
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to copy file %s to tar: %w", path, err)
			}
		}

		return nil
	})

	if err != nil {
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
		if info.IsDir() && info.Name() == ".git" {
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
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
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
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to copy directory contents: %w", err)
	}

	return tempDir, nil
}

// copyFile copies a single file from src to dst with the specified mode
func copyFile(src, dst string, mode os.FileMode) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}
