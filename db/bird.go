package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"os"
	"time"
)

type HourlyObservations map[int]int

func (h HourlyObservations) Value() (driver.Value, error) {
	return json.Marshal(h)
}

func (h *HourlyObservations) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, h)
}

type BirdData struct {
	ID                 uint   `gorm:"primary_key;auto_increment"`
	Name               string `gorm:"not null"`
	FeederToken        string `gorm:"not null;index"`
	CreatedAt          time.Time
	HourlyObservations HourlyObservations `gorm:"type:json"`
}

func ConnectDB() *gorm.DB {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})

	if err != nil {
		panic("failed to connect to database")
	}
	fmt.Println("Successfully connected to database")
	migrate(database)

	return database
}

func migrate(database *gorm.DB) {
	_ = database.AutoMigrate(&BirdData{})
}
