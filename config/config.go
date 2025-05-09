package config

import "os"

type Config struct {
	GRPCPort string
}

func Load() *Config {
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50051"
	}
	return &Config{
		GRPCPort: port,
	}
}
