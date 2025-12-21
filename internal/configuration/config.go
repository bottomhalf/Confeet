package configuration

import (
	"encoding/json"
	"os"
)

type MongoConfig struct {
	Uri                string `json:"uri"`
	Database           string `json:"database"`
	MessagesCollection string `json:"messagesCollection"`
	SessionsCollection string `json:"sessionsCollection"`
	ProductsCollection string `json:"productsCollection"`
	SocketRoute        string `json:"socketRoute"`
	UUIDNamespace      string `json:"uuid_namespace"`
}

type DatabaseConfig struct {
	Dsn string `json:"dsn"`
}

type ServerConfig struct {
	AppPort    int `json:"app_port"`
	SocketPort int `json:"socket_port"`
}

type Config struct {
	Database     DatabaseConfig `json:"mysql"`
	ChatDatabase MongoConfig    `json:"mongo"`
	Server       ServerConfig   `json:"server"`
}

func LoadConfig(config_path string) (*Config, error) {
	file, err := os.ReadFile(config_path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
