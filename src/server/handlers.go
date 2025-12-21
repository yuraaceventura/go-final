package server

import (
	"archive/zip"
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

	successfulInserts := 0
	for i := startIndex; i < len(records); i++ {
		record := records[i]
		if len(record) >= 5 {
			_, err := strconv.Atoi(record[0])
			if err != nil {
				log.Printf("Invalid ID in record %d: %v", i, err)
				continue
			}

			name := record[1]
			category := record[2]
			price, err := strconv.ParseFloat(record[3], 64)
			if err != nil {
				log.Printf("Invalid price in record %d: %v", i, err)
				continue
			}

			createDate, err := time.Parse("2006-01-02", record[4])
			if err != nil {
				createDate = time.Now()
				log.Printf("Invalid date format in record %d, using current date: %v", i, err)
			}

			if err := db.InsertPrice(name, category, price, createDate); err != nil {
				log.Printf("Failed to insert price at line %d: %v", i, err)
			} else {
				successfulInserts++
			}
		}
	}

	stats, err := db.GetStats()
	if err != nil {
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	response := UploadResponse{
		TotalItems:      stats["total_items"].(int),
		TotalCategories: stats["total_categories"].(int),
		TotalPrice:      stats["total_price"].(float64),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleGetPrices(w http.ResponseWriter, r *http.Request) {
	prices, err := db.GetAllPrices()
	if err != nil {
		http.Error(w, "Failed to get prices", http.StatusInternalServerError)
		return
	}

	csvFile, err := os.CreateTemp("", "data-*.csv")
	if err != nil {
		http.Error(w, "Failed to create CSV file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(csvFile.Name())
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	writer.Write([]string{"ID", "Name", "Category", "Price", "CreatedAt"})
	for _, price := range prices {
		record := []string{
			strconv.Itoa(price.ID),
			price.Name,
			price.Category,
			strconv.FormatFloat(price.Price, 'f', 2, 64),
			price.CreatedAt.Format("2006-01-02"),
		}
		writer.Write(record)
	}
	writer.Flush()

	zipFile, err := os.CreateTemp("", "prices-*.zip")
	if err != nil {
		http.Error(w, "Failed to create zip file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(zipFile.Name())
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	fileInArchive, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Failed to create file in archive", http.StatusInternalServerError)
		return
	}

	csvContent, err := os.ReadFile(csvFile.Name())
	if err != nil {
		http.Error(w, "Failed to read CSV file", http.StatusInternalServerError)
		return
	}

	fileInArchive.Write(csvContent)
	zipWriter.Close()

	zipData, err := os.ReadFile(zipFile.Name())
	if err != nil {
		http.Error(w, "Failed to read zip file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=prices.zip")
	w.Write(zipData)
}
