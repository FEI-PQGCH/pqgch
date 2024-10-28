package shared

import (
	"encoding/json"
	"log"
	"os"
)

type UserConfig struct {
	LeadAddr string `json:"leadAddr"`
	Name     string `json:"name"`
}

type ServConfig struct {
	ServAddrs []string `json:"servers"`
	Index     int      `json:"index"`
}

func GetUserConfig(path string) UserConfig {
	config := UserConfig{}

	configFile, err := os.Open(path)
	if err != nil {
		log.Fatalf("Error opening config file at %s: %v", path, err)
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}

	return config
}

func GetServConfig(path string) ServConfig {
	config := ServConfig{}

	configFile, err := os.Open(path)
	if err != nil {
		log.Fatalf("Error opening config file at %s: %v", path, err)
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}
	return config
}

func (c *ServConfig) GetLeftNeighbor() string {
	if c.Index == 0 {
		return c.ServAddrs[len(c.ServAddrs)-1]
	}
	return c.ServAddrs[c.Index-1]
}

func (c *ServConfig) GetCurrentServer() string {
	return c.ServAddrs[c.Index]
}
