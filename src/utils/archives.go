package utils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

func ExtractZip(zipPath string) (string, error) {
	destDir, err := os.MkdirTemp("", "extracted-*")
	if err != nil {
		return "", err
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		destPath := filepath.Join(destDir, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(destPath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
			return "", err
		}

		destFile, err := os.Create(destPath)
		if err != nil {
			return "", err
		}
		defer destFile.Close()

		srcFile, err := file.Open()
		if err != nil {
			return "", err
		}
		defer srcFile.Close()

		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			return "", err
		}
	}

	return destDir, nil
}

func ExtractTar(tarPath string) (string, error) {
	destDir, err := os.MkdirTemp("", "extracted-*")
	if err != nil {
		return "", err
	}

	file, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var reader io.Reader = file

	// Проверяем, сжат ли tar.gz
	if filepath.Ext(tarPath) == ".gz" {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return "", err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		destPath := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, os.ModePerm); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
				return "", err
			}

			destFile, err := os.Create(destPath)
			if err != nil {
				return "", err
			}
			defer destFile.Close()

			if _, err := io.Copy(destFile, tarReader); err != nil {
				return "", err
			}
		}
	}

	return destDir, nil
}
