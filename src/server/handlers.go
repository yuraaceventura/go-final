package server

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"project/src/db"
	"project/src/utils"
)

type UploadResponse struct {
	TotalItems      int     `json:"total_items"`
	TotalCategories int     `json:"total_categories"`
	TotalPrice      float64 `json:"total_price"`
}

func PricesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		handlePostPrices(w, r)
	case http.MethodGet:
		handleGetPrices(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handlePostPrices(w http.ResponseWriter, r *http.Request) {
	archiveType := r.URL.Query().Get("type")
	if archiveType == "" {
		archiveType = "zip"
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	tmpFile, err := os.CreateTemp("", "upload-*")
	if err != nil {
		http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(body); err != nil {
		http.Error(w, "Failed to write temp file", http.StatusInternalServerError)
		return
	}

	var extractedPath string
	switch strings.ToLower(archiveType) {
	case "zip":
		extractedPath, err = utils.ExtractZip(tmpFile.Name())
	case "tar":
		extractedPath, err = utils.ExtractTar(tmpFile.Name())
	default:
		http.Error(w, "Unsupported archive type", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to extract archive: %v", err), http.StatusBadRequest)
		return
	}
	defer os.RemoveAll(extractedPath)

	records, err := utils.ReadCSVFiles(extractedPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read CSV: %v", err), http.StatusBadRequest)
		return
	}

	startIndex := 0
	if len(records) > 0 && strings.ToLower(records[0][0]) == "id" {
		startIndex = 1
	}

	validRecords := make([]db.PriceRecord, 0)
	validationErrors := make([]string, 0)

	for i := startIndex; i < len(records); i++ {
		record := records[i]
		if len(record) < 5 {
			validationErrors = append(validationErrors, fmt.Sprintf("Record %d: insufficient fields", i))
			continue
		}

		id, err := strconv.Atoi(record[0])
		if err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("Record %d: invalid ID: %v", i, err))
			continue
		}

		name := strings.TrimSpace(record[1])
		if name == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("Record %d: empty name", i))
			continue
		}

		category := strings.TrimSpace(record[2])
		price, err := strconv.ParseFloat(record[3], 64)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("Record %d: invalid price: %v", i, err))
			continue
		}

		if price < 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("Record %d: negative price", i))
			continue
		}

		var createDate time.Time
		if dateStr := strings.TrimSpace(record[4]); dateStr != "" {
			createDate, err = time.Parse("2006-01-02", dateStr)
			if err != nil {
				validationErrors = append(validationErrors, fmt.Sprintf("Record %d: invalid date format: %v", i, err))
				continue
			}
		} else {
			createDate = time.Now()
		}

		validRecords = append(validRecords, db.PriceRecord{
			ID:       id,
			Name:     name,
			Category: category,
			Price:    price,
			Date:     createDate,
		})
	}

	if len(validationErrors) > 0 {
		log.Printf("Validation errors: %v", validationErrors)
	}

	response, err := db.InsertPricesWithStats(validRecords)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to insert prices: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Тут убрал временные файлы
func handleGetPrices(w http.ResponseWriter, r *http.Request) {
	prices, err := db.GetAllPrices()
	if err != nil {
		http.Error(w, "Failed to get prices", http.StatusInternalServerError)
		return
	}

	var csvBuffer bytes.Buffer
	writer := csv.NewWriter(&csvBuffer)

	if err := writer.Write([]string{"ID", "Name", "Category", "Price", "CreatedAt"}); err != nil {
		http.Error(w, "Failed to write CSV header", http.StatusInternalServerError)
		return
	}

	for _, price := range prices {
		record := []string{
			strconv.Itoa(price.ID),
			price.Name,
			price.Category,
			strconv.FormatFloat(price.Price, 'f', 2, 64),
			price.CreatedAt.Format("2006-01-02"),
		}
		if err := writer.Write(record); err != nil {
			http.Error(w, "Failed to write CSV record", http.StatusInternalServerError)
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		http.Error(w, "Failed to flush CSV", http.StatusInternalServerError)
		return
	}

	var zipBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuffer)

	fileInArchive, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Failed to create file in archive", http.StatusInternalServerError)
		return
	}

	if _, err := fileInArchive.Write(csvBuffer.Bytes()); err != nil {
		http.Error(w, "Failed to write to archive", http.StatusInternalServerError)
		return
	}

	if err := zipWriter.Close(); err != nil {
		http.Error(w, "Failed to close archive", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=prices.zip")

	if _, err := w.Write(zipBuffer.Bytes()); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}
