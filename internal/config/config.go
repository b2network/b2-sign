package config

import (
	"github.com/caarlos0/env/v6"
)

// Config is the global config.
type Config struct {
	// B2NodeChainID defines the b2 node chain id
	B2NodeChainID string `env:"B2_NODE_CHAIN_ID"`
	// B2NodeGRPCHost defines the b2 node grpc host
	B2NodeGRPCHost string `env:"B2_NODE_GRPC_HOST" envDefault:"127.0.0.1"`
	// B2NodeGRPCPort defines the b2 node grpc port
	B2NodeGRPCPort uint32 `env:"B2_NODE_GRPC_PORT" envDefault:"9090"`
	// B2NodeDenom defines the b2 node denom
	B2NodeDenom string ` env:"B2_NODE_DENOM" envDefault:"aphoton"`
	// B2NodeUnsignedTxLimit defines the pull b2 node unsigned data length
	B2NodeUnsignedTxLimit uint64 `env:"B2_NODE_UNSIGNED_TX_LIMIT" envDefault:"100"`
}

// LoadConfig load config from environment.
func LoadConfig() (*Config, error) {
	config := Config{}
	if err := env.Parse(&config); err != nil {
		return nil, err
	}
	return &config, nil
}
