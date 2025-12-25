package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

type Price struct {
	ID        int
	Name      string
	Category  string
	Price     float64
	CreatedAt time.Time
}

type PriceRecord struct {
	ID       int
	Name     string
	Category string
	Price    float64
	Date     time.Time
}

func getDBConfig() (host, port, user, password, dbname string) {
	host = os.Getenv("POSTGRES_HOST")
	port = os.Getenv("POSTGRES_PORT")
	user = os.Getenv("POSTGRES_USER")
	password = os.Getenv("POSTGRES_PASS")
	dbname = os.Getenv("POSTGRES_DB")
	return
}

func InitDB() error {
	host, port, user, password, dbname := getDBConfig()

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var err error
	DB, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		return err
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(25)
	DB.SetConnMaxLifetime(5 * time.Minute)

	for i := 0; i < 10; i++ {
		err = DB.Ping()
		if err == nil {
			break
		}
		log.Printf("Attempt %d: Failed to connect to database: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to connect after retries: %v", err)
	}

	log.Println("Successfully connected to database")

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS prices (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		category VARCHAR(100),
		price DECIMAL(10,2),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	_, err = DB.Exec(createTableQuery)
	if err != nil {
		return err
	}

	return nil
}

func CloseDB() {
	if DB != nil {
		DB.Close()
	}
}

func InsertPricesWithStats(records []PriceRecord) (map[string]interface{}, error) {
	tx, err := DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare("INSERT INTO prices (name, category, price, created_at) VALUES ($1, $2, $3, $4)")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	successfulInserts := 0
	for _, record := range records {
		_, err := stmt.Exec(record.Name, record.Category, record.Price, record.Date)
		if err != nil {
			log.Printf("Failed to insert record %d: %v", record.ID, err)
			continue
		}
		successfulInserts++
	}

	var totalCategories int
	var totalPrice float64
	query := `
		SELECT  
			COUNT(DISTINCT category) as total_categories, 
			COALESCE(SUM(price), 0) as total_price 
		FROM prices
	`
	row := tx.QueryRow(query)
	if err := row.Scan(&totalCategories, &totalPrice); err != nil {
		return nil, fmt.Errorf("failed to fetch stats: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	stats := map[string]interface{}{
		"total_items":      successfulInserts,
		"total_categories": totalCategories,
		"total_price":      totalPrice,
	}

	return stats, nil
}

func GetAllPrices() ([]Price, error) {
	rows, err := DB.Query("SELECT id, name, category, price, created_at FROM prices ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prices []Price
	for rows.Next() {
		var p Price
		err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.Price, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		prices = append(prices, p)
	}

	// Проверяем ошибки после закрытия rows согласно документации
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return prices, nil
}
