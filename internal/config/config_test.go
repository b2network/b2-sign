package config_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/b2network/b2-sign/internal/config"
)

func TestConfigEnv(t *testing.T) {
	os.Setenv("B2_NODE_CHAIN_ID", "b2node_9000")
	os.Setenv("B2_NODE_GRPC_HOST", "127.0.0.1")
	os.Setenv("B2_NODE_GRPC_PORT", "9090")
	os.Setenv("B2_NODE_DENOM", "aphoton")
	os.Setenv("B2_NODE_UNSIGNED_TX_LIMIT", "100")
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, &config.Config{
		B2NodeGRPCHost:        "127.0.0.1",
		B2NodeGRPCPort:        9090,
		B2NodeDenom:           "aphoton",
		B2NodeChainID:         "b2node_9000",
		B2NodeUnsignedTxLimit: 100,
	}) {
		t.Fatal("config mismatch")
	}
}
