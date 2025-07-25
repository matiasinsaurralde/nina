package builder

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"

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
			return err
		}
		b.logger.Info("Bundle cleanup completed", "temp_dir", b.tempDir)
	}
	return nil
}

// NewBundle creates a new bundle from the given request.
func NewBundle(req *types.BuildRequest, log *logger.Logger) (bundle *Bundle, err error) {
	bundle = &Bundle{
		logger: log,
	}

	log.Info("Starting bundle extraction", "app_name", req.AppName, "bundle_size_bytes", len(req.BundleContents))

	bundle.Contents, err = base64.StdEncoding.DecodeString(req.BundleContents)
	if err != nil {
		log.Error("Failed to decode base64 bundle contents", "app_name", req.AppName, "error", err)
		return nil, err
	}
	log.Info("Base64 decoded successfully", "app_name", req.AppName, "decoded_size_bytes", len(bundle.Contents))

	// Uncompress the contents (they are a gzipped tar file):
	gz, err := gzip.NewReader(bytes.NewReader(bundle.Contents))
	if err != nil {
		log.Error("Failed to create gzip reader", "app_name", req.AppName, "error", err)
		return nil, err
	}
	defer gz.Close()
	log.Info("Gzip reader created successfully", "app_name", req.AppName)

	// Unpack the contents into a temporary directory:
	bundle.tempDir, err = os.MkdirTemp("", "nina-bundle")
	if err != nil {
		log.Error("Failed to create temporary directory", "app_name", req.AppName, "error", err)
		return nil, err
	}
	log.Info("Temporary directory created", "app_name", req.AppName, "temp_dir", bundle.tempDir)

	// Extract the tar contents
	tarReader := tar.NewReader(gz)
	fileCount := 0
	dirCount := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error("Failed to read tar entry", "app_name", req.AppName, "error", err)
			return nil, err
		}

		// Create the full path for the file
		target := filepath.Join(bundle.tempDir, header.Name)

		// Handle different types of tar entries
		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(target, 0755); err != nil {
				log.Error("Failed to create directory", "app_name", req.AppName, "path", target, "error", err)
				return nil, err
			}
			dirCount++
		case tar.TypeReg:
			// Create file
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				log.Error("Failed to create parent directory", "app_name", req.AppName, "path", filepath.Dir(target), "error", err)
				return nil, err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				log.Error("Failed to create file", "app_name", req.AppName, "path", target, "error", err)
				return nil, err
			}
			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				log.Error("Failed to write file contents", "app_name", req.AppName, "path", target, "error", err)
				return nil, err
			}
			file.Close()
			fileCount++
		}
	}

	log.Info("Bundle extraction completed", "app_name", req.AppName, "files_extracted", fileCount, "directories_created", dirCount, "temp_dir", bundle.tempDir)

	// TODO: validate the bundle contents
	// Keep the request object just in case
	bundle.req = req
	return bundle, nil
}
