package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/config"
)

// CreateBackup creates a .tar.gz backup of the ~/.hermes directory.
func CreateBackup(outputPath string) error {
	hermesDir := config.HermesHome()

	if outputPath == "" {
		outputPath = fmt.Sprintf("hermes-backup-%s.tar.gz", time.Now().Format("20060102-150405"))
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	count := 0
	err = filepath.Walk(hermesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Skip large files and sensitive patterns
		if info.Size() > 100*1024*1024 {
			return nil
		}
		base := filepath.Base(path)
		if base == ".env" || strings.HasSuffix(base, ".key") {
			return nil
		}

		relPath, err := filepath.Rel(hermesDir, path)
		if err != nil {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return nil
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			io.Copy(tarWriter, f)
			count++
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walk hermes dir: %w", err)
	}

	slog.Info("Backup created", "path", outputPath, "files", count)
	return nil
}

// RestoreBackup extracts a backup archive to ~/.hermes.
func RestoreBackup(archivePath string) error {
	hermesDir := config.HermesHome()

	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	count := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		targetPath := filepath.Join(hermesDir, filepath.FromSlash(header.Name))

		// Path traversal check
		if !strings.HasPrefix(targetPath, hermesDir) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(targetPath, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(targetPath), 0755)
			outFile, err := os.Create(targetPath)
			if err != nil {
				continue
			}
			io.Copy(outFile, tarReader)
			outFile.Close()
			count++
		}
	}

	slog.Info("Backup restored", "path", archivePath, "files", count)
	return nil
}
