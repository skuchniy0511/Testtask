package config

import (
	"gopkg.in/yaml.v2"
	"os"
)

type Config struct {
	AmqpUrl               string `yaml:"AMQP_SERVER_URL"`
	LogFilePath           string `yaml:"logFile"`
	ClientsInputPath      string `yaml:"clientsInputPath"`
	ServerWaitTimeSeconds int64  `yaml:"serverWaitTimeSeconds"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Substitute from environemental vars
	confContent := []byte(os.ExpandEnv(string(data)))

	config := &Config{}

	err = yaml.Unmarshal(confContent, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
