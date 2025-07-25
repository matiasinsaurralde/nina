package builder

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

	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/types"
)

// Bundle represents a bundle of contents.
type Bundle struct {
	Contents []byte
	req      *types.BuildRequest
	tempDir  string
	logger   *logger.Logger
}

// GetTempDir returns the temporary directory where the bundle was extracted
func (b *Bundle) GetTempDir() string {
	return b.tempDir
}

// GetLogger returns the logger instance
func (b *Bundle) GetLogger() *logger.Logger {
	return b.logger
}

// GetRequest returns the original build request
func (b *Bundle) GetRequest() *types.BuildRequest {
	return b.req
}

// Cleanup removes the temporary directory and its contents
func (b *Bundle) Cleanup() error {
	if b.tempDir != "" {
		b.logger.Info("Cleaning up bundle", "temp_dir", b.tempDir)
		if err := os.RemoveAll(b.tempDir); err != nil {
			b.logger.Error("Failed to cleanup bundle", "temp_dir", b.tempDir, "error", err)
			return fmt.Errorf("failed to cleanup bundle: %w", err)
		}
		b.logger.Info("Bundle cleanup completed", "temp_dir", b.tempDir)
	}
	return nil
}

// decodeBundleContents decodes the base64 bundle contents
func decodeBundleContents(req *types.BuildRequest, log *logger.Logger) ([]byte, error) {
	log.Info("Starting bundle extraction", "app_name", req.AppName, "bundle_size_bytes", len(req.BundleContents))

	contents, err := base64.StdEncoding.DecodeString(req.BundleContents)
	if err != nil {
		log.Error("Failed to decode base64 bundle contents", "app_name", req.AppName, "error", err)
		return nil, fmt.Errorf("failed to decode base64 bundle contents: %w", err)
	}
	log.Info("Base64 decoded successfully", "app_name", req.AppName, "decoded_size_bytes", len(contents))
	return contents, nil
}

// createGzipReader creates a gzip reader for the bundle contents
func createGzipReader(contents []byte, req *types.BuildRequest, log *logger.Logger) (*gzip.Reader, error) {
	gz, err := gzip.NewReader(bytes.NewReader(contents))
	if err != nil {
		log.Error("Failed to create gzip reader", "app_name", req.AppName, "error", err)
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	log.Info("Gzip reader created successfully", "app_name", req.AppName)
	return gz, nil
}

// createTempDirectory creates a temporary directory for bundle extraction
func createTempDirectory(req *types.BuildRequest, log *logger.Logger) (string, error) {
	tempDir, err := os.MkdirTemp("", "nina-bundle")
	if err != nil {
		log.Error("Failed to create temporary directory", "app_name", req.AppName, "error", err)
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	log.Info("Temporary directory created", "app_name", req.AppName, "temp_dir", tempDir)
	return tempDir, nil
}

// validateTargetPath validates that the target path is within the temp directory
func validateTargetPath(target, tempDir string) error {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	absTempDir, err := filepath.Abs(tempDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute temp directory path: %w", err)
	}

	if !strings.HasPrefix(absTarget, absTempDir) {
		return fmt.Errorf("invalid file path")
	}
	return nil
}

// extractTarEntry extracts a single tar entry
func extractTarEntry(header *tar.Header, tarReader *tar.Reader, tempDir string, log *logger.Logger) (fileCount, dirCount int, err error) {
	//nolint: gosec
	target := filepath.Join(tempDir, header.Name)

	if err := validateTargetPath(target, tempDir); err != nil {
		return 0, 0, fmt.Errorf("failed to validate path for %s: %w", header.Name, err)
	}

	if header.FileInfo().IsDir() {
		if err := os.MkdirAll(target, 0o750); err != nil {
			return 0, 0, fmt.Errorf("failed to create directory %s: %w", target, err)
		}
		dirCount++
	} else {
		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return 0, 0, fmt.Errorf("failed to create parent directories for %s: %w", target, err)
		}

		// Create the file with proper permissions
		//nolint: gosec
		file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to create file %s: %w", target, err)
		}

		// Limit the size to prevent decompression bomb
		limitedReader := io.LimitReader(tarReader, 10*1024*1024) // 10MB limit
		if _, err := io.Copy(file, limitedReader); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				log.Error("Failed to close file after copy error", "error", closeErr)
			}
			return 0, 0, fmt.Errorf("failed to copy file content: %w", err)
		}
		if err := file.Close(); err != nil {
			return 0, 0, fmt.Errorf("failed to close file: %w", err)
		}
		fileCount++
	}

	return fileCount, dirCount, nil
}

// extractTarContents extracts all contents from the tar archive
func extractTarContents(tarReader *tar.Reader, tempDir string, req *types.BuildRequest, log *logger.Logger) error {
	fileCount := 0
	dirCount := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error("Failed to read tar entry", "app_name", req.AppName, "error", err)
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		fc, dc, err := extractTarEntry(header, tarReader, tempDir, log)
		if err != nil {
			return err
		}
		fileCount += fc
		dirCount += dc
	}

	log.Info("Bundle extraction completed", "app_name", req.AppName, "files_extracted", fileCount,
		"directories_created", dirCount, "temp_dir", tempDir)
	return nil
}

// NewBundle creates a new bundle from the given request.
func NewBundle(req *types.BuildRequest, log *logger.Logger) (bundle *Bundle, err error) {
	bundle = &Bundle{
		logger: log,
	}

	// Decode bundle contents
	bundle.Contents, err = decodeBundleContents(req, log)
	if err != nil {
		return nil, err
	}

	// Create gzip reader
	gz, err := createGzipReader(bundle.Contents, req, log)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := gz.Close(); closeErr != nil {
			log.Error("Failed to close gzip reader", "app_name", req.AppName, "error", closeErr)
		}
	}()

	// Create temporary directory
	bundle.tempDir, err = createTempDirectory(req, log)
	if err != nil {
		return nil, err
	}

	// Extract tar contents
	tarReader := tar.NewReader(gz)
	if err := extractTarContents(tarReader, bundle.tempDir, req, log); err != nil {
		return nil, err
	}

	// Keep the request object just in case
	bundle.req = req
	return bundle, nil
}
