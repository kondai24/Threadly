package database

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"Threadly/internal/domain/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const gormSlowThreshold = 200 * time.Millisecond

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
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=True",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.DB,
	)
	db, err := gorm.Open(mysql.Open(dsn), NewGORMConfig())
	if err != nil {
		return nil, err
	}
	if err := autoMigrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

func NewGORMConfig() *gorm.Config {
	return &gorm.Config{
		Logger: newGORMLogger(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gormSlowThreshold,
		),
		TranslateError: true,
	}
}

func newGORMLogger(writer logger.Writer, slowThreshold time.Duration) logger.Interface {
	return logger.New(writer, logger.Config{
		SlowThreshold:        slowThreshold,
		Colorful:             false,
		ParameterizedQueries: true,
		LogLevel:             logger.Warn,
	})
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.User{}, &models.Post{})
}
