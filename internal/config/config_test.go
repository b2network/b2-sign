package config_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/b2network/b2-sign/internal/config"
)

func TestConfigEnv(t *testing.T) {
	os.Setenv("UNSIGNED_API", "https://127.0.0.1/api/unsigned")
	os.Setenv("SIGNED_API", "https://127.0.0.1:/api/signed")
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, &config.Config{
		UnsignedAPI: "https://127.0.0.1/api/unsigned",
		SignedAPI:   "https://127.0.0.1:/api/signed",
	}) {
		t.Fatal("config mismatch")
	}
}
