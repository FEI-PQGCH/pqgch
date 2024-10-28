package shared

import (
	"encoding/json"
	"log"
	"os"
)

type ServerConfig struct {
    Address string `json:"address"`
    Port    int    `json:"port"`
}

type Config struct {
    Database struct {
        Host     string `json:"host"`
        Password string `json:"password"`
        User     string `json:"user"`
        Port     int    `json:"port"`
    } `json:"database"`
    Token         string         `json:"token"`
    Servers       []ServerConfig `json:"servers"`
    SelfAddress   string         `json:"self_address"`  
    LeftNeighbor  string         `json:"left_neighbor"`  
    RightNeighbor string         `json:"right_neighbor"` 
}

func GetConfig() Config {
    config := Config{
        Database: struct {
            Host     string `json:"host"`
            Password string `json:"password"`
            User     string `json:"user"`
            Port     int    `json:"port"`
        }{
            Port: 5432,
        },
    }

    configPath := "./.config/your_config.json"

    configFile, err := os.Open(configPath)
    if err != nil {
        log.Fatalf("Error opening config file at %s: %v", configPath, err)
    }
    defer configFile.Close()

    jsonParser := json.NewDecoder(configFile)
    err = jsonParser.Decode(&config)
    if err != nil {
        log.Fatalf("Error parsing config file: %v", err)
    }
    return config
}

func GetConfigFromPath(path string) Config {
	config := Config{
	}

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