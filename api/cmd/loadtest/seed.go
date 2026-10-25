package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	"Threadly/internal/infra/auth"
	"Threadly/internal/infra/database"
	"Threadly/internal/loadtest"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// seedは、専用Composeのtool containerだけで実行する。接続先は専用DBに固定し、
// 任意のDB_HOST・DB名・DSNへフォールバックしない。verifyも同じ接続先を使う。
func seed(args []string) int {
	flags := flag.NewFlagSet("seed", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dataset := flags.String("dataset", "", "")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		invalidInput()
	}
	fixture, err := loadtest.GenerateFixture(*dataset)
	if err != nil {
		invalidInput()
	}
	dbPassword := os.Getenv("LOADTEST_DB_PASSWORD")
	userPassword := os.Getenv("LOADTEST_USER_PASSWORD")
	if dbPassword == "" || userPassword == "" {
		return fail(&loadtest.Failure{Code: 5, Reason: "secrets_missing"})
	}
	db, err := openLoadtestDB(dbPassword)
	if err != nil {
		return fail(err)
	}
	summary, err := loadtest.LoadFixture(context.Background(), db, fixture, auth.NewArgon2idHasher(), userPassword)
	if err != nil {
		// 初期化済みDBと読み戻し不一致は準備の不整合として、接続・投入障害と区別する。
		if errors.Is(err, loadtest.ErrAlreadyInitialized) {
			return fail(&loadtest.Failure{Code: 5, Reason: "already_initialized"})
		}
		if errors.Is(err, loadtest.ErrFixtureMismatch) {
			return fail(&loadtest.Failure{Code: 5, Reason: "fixture_mismatch"})
		}
		return fail(&loadtest.Failure{Code: 4, Reason: "seed_failed"})
	}
	if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
		return 5
	}
	return 0
}

// verifyは、run直前に専用DBの件数とchecksumを読み戻し、準備時の記録と照合できる形で出力する。
func verify(args []string) int {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dataset := flags.String("dataset", "", "")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		invalidInput()
	}
	if _, err := loadtest.NewPlan("browse", *dataset); err != nil {
		invalidInput()
	}
	dbPassword := os.Getenv("LOADTEST_DB_PASSWORD")
	if dbPassword == "" {
		return fail(&loadtest.Failure{Code: 5, Reason: "secrets_missing"})
	}
	db, err := openLoadtestDB(dbPassword)
	if err != nil {
		return fail(err)
	}
	summary, err := loadtest.ReadFixtureSummary(context.Background(), db, *dataset)
	if err != nil {
		return fail(&loadtest.Failure{Code: 4, Reason: "verify_failed"})
	}
	if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
		return 5
	}
	return 0
}

func openLoadtestDB(password string) (*gorm.DB, error) {
	config := mysqldriver.NewConfig()
	config.User = "root"
	config.Passwd = password
	config.Net = "tcp"
	config.Addr = "loadtest-db:3306"
	config.DBName = "threadly_loadtest"
	config.ParseTime = true
	db, err := gorm.Open(mysql.Open(config.FormatDSN()), database.NewGORMConfig())
	if err != nil {
		return nil, &loadtest.Failure{Code: 4, Reason: "database_unavailable"}
	}
	return db, nil
}
