package utils

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
)

func ReadCSVFiles(dir string) ([][]string, error) {
	var allRecords [][]string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".csv" {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			reader := csv.NewReader(file)
			// Пропускаем заголовок если есть
			if _, err := reader.Read(); err != nil && err != io.EOF {
				return err
			}

			for {
				record, err := reader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					continue // Пропускаем некорректные строки
				}

				allRecords = append(allRecords, record)
			}
		}

		return nil
	})

	return allRecords, err
}
