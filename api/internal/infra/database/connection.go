package database

import (
	"fmt"
	"os"
	"strconv"
	"Threadly/internal/domain/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type DBConfig struct {
	User     string
	Password string
	Host     string
	Port     int
	DB       string
}

func getDBConfig() DBConfig {
	port, err := strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		port = 3306
	}
	return DBConfig{
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Host:     os.Getenv("DB_HOST"),
		Port:     port,
		DB:       os.Getenv("DB"),
	}
}

func ConnectionDB() (*gorm.DB, error) {
	config := getDBConfig()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=True", config.User, config.Password, config.Host, config.Port, config.DB)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&models.Post{}); err != nil {
		return nil, err
	}
	return db, nil
}
