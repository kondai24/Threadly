package database

import (
	"strings"
	"testing"
)

func TestParseDBConfig_ReadsAllSettings(t *testing.T) {
	env := validDBEnv()

	config, err := parseDBConfig(mapGetenv(env))
	if err != nil {
		t.Fatalf("parseDBConfig() error = %v", err)
	}
	want := DBConfig{User: "app", Password: "secret", Host: "db", Port: 3307, DB: "threadly"}
	if config != want {
		t.Fatalf("parseDBConfig() = %+v, want %+v", config, want)
	}
}

func TestParseDBConfig_RejectsMissingRequiredSetting(t *testing.T) {
	for _, key := range []string{"DB_USER", "DB_PASSWORD", "DB_HOST", "DB_PORT", "DB"} {
		t.Run(key, func(t *testing.T) {
			env := validDBEnv()
			delete(env, key)

			_, err := parseDBConfig(mapGetenv(env))
			if err == nil || err.Error() != "database config "+key+": required" {
				t.Fatalf("parseDBConfig() error = %v, want %s required", err, key)
			}
		})
	}
}

func TestParseDBConfig_RejectsInvalidPortWithoutRawValue(t *testing.T) {
	for _, port := range []string{"abc", "0", "65536", "-1"} {
		t.Run(port, func(t *testing.T) {
			env := validDBEnv()
			env["DB_PORT"] = port

			_, err := parseDBConfig(mapGetenv(env))
			if err == nil || err.Error() != "database config DB_PORT: invalid port" {
				t.Fatalf("parseDBConfig() error = %v, want invalid port", err)
			}
		})
	}
}

func TestParseDBConfig_ErrorDoesNotExposePassword(t *testing.T) {
	env := validDBEnv()
	env["DB_PASSWORD"] = "sensitive-password"
	delete(env, "DB")

	_, err := parseDBConfig(mapGetenv(env))
	if err == nil || strings.Contains(err.Error(), "sensitive-password") {
		t.Fatalf("parseDBConfig() error = %v, want error without password", err)
	}
}

func validDBEnv() map[string]string {
	return map[string]string{
		"DB_USER":     "app",
		"DB_PASSWORD": "secret",
		"DB_HOST":     "db",
		"DB_PORT":     "3307",
		"DB":          "threadly",
	}
}

func mapGetenv(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}
