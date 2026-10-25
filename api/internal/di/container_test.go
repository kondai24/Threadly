package di

import (
	"strings"
	"testing"
)

// os.Getenvではなく引数のgetenvが使われることを、DB接続前の設定検証で確認する。
func TestResolveRouter_UsesInjectedGetenv(t *testing.T) {
	env := map[string]string{"JWT_SECRET": strings.Repeat("s", 32)}
	t.Setenv("DB_USER", "from-process-env")
	t.Setenv("JWT_SECRET", "")

	container, err := NewContainer(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}
	_, err = ResolveRouter(container)
	if err == nil || !strings.Contains(err.Error(), "database config DB_USER: required") {
		t.Fatalf("ResolveRouter() error = %v, want DB_USER required", err)
	}
}
