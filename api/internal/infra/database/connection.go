package database

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

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

// configErrorは、raw値を含めず設定項目名と分類だけを返す。DB_PASSWORDをログへ出さないため。
type configError struct {
	key    string
	reason string
}

func (e *configError) Error() string {
	return fmt.Sprintf("database config %s: %s", e.key, e.reason)
}

// parseDBConfigは、Makefileのmigrationと同じく5項目すべてを必須とし、
// 不正なportを既定値へ置き換えず起動時に拒否する。
func parseDBConfig(getenv func(string) string) (DBConfig, error) {
	for _, key := range []string{"DB_USER", "DB_PASSWORD", "DB_HOST", "DB_PORT", "DB"} {
		if getenv(key) == "" {
			return DBConfig{}, &configError{key: key, reason: "required"}
		}
	}
	port, err := strconv.Atoi(getenv("DB_PORT"))
	if err != nil || port < 1 || port > 65535 {
		return DBConfig{}, &configError{key: "DB_PORT", reason: "invalid port"}
	}
	return DBConfig{
		User:     getenv("DB_USER"),
		Password: getenv("DB_PASSWORD"),
		Host:     getenv("DB_HOST"),
		Port:     port,
		DB:       getenv("DB"),
	}, nil
}

// ConnectionDBは、起動処理から渡されたgetenvでDB設定を読み、MySQLへ接続する。
func ConnectionDB(getenv func(string) string) (*gorm.DB, error) {
	config, err := parseDBConfig(getenv)
	if err != nil {
		return nil, err
	}
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
