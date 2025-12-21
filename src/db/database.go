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

func InsertPrice(name, category string, price float64, created_at time.Time) error {
	query := `INSERT INTO prices (name, category, price, created_at) VALUES ($1, $2, $3, $4)`
	_, err := DB.Exec(query, name, category, price, created_at)
	return err
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

	return prices, nil
}

func GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total items
	var totalItems int
	err := DB.QueryRow("SELECT COUNT(*) FROM prices").Scan(&totalItems)
	if err != nil {
		return nil, err
	}
	stats["total_items"] = totalItems

	// Total categories
	var totalCategories int
	err = DB.QueryRow("SELECT COUNT(DISTINCT category) FROM prices").Scan(&totalCategories)
	if err != nil {
		return nil, err
	}
	stats["total_categories"] = totalCategories

	// Total price
	var totalPrice float64
	err = DB.QueryRow("SELECT COALESCE(SUM(price), 0) FROM prices").Scan(&totalPrice)
	if err != nil {
		return nil, err
	}
	stats["total_price"] = totalPrice

	return stats, nil
}
