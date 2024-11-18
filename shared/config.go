package shared

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
)

type BaseConfig struct {
	Names      []string `json:"names"`
	Index      int      `json:"index"`
	PublicKeys []string `json:"publicKeys"`
}

type UserConfig struct {
	BaseConfig
	LeadAddr  string `json:"leadAddr"`
	SecretKey string `json:"secretKey"`
}

type ServConfig struct {
	BaseConfig
	ServAddrs []string `json:"servers"`
}

type SessionInfo struct {
	ConfigUser   UserConfig
	ConfigServer ServConfig
	TkRight      []byte
	EskaRight    []byte
	KeyLeft      [32]byte
	KeyRight     [32]byte
	Xs           [][32]byte
	SharedSecret [32]byte
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

func (c *BaseConfig) GetName() string {
	return c.Names[c.Index]
}

func (c *UserConfig) GetDecodedSecretKey() []byte {
	decodedSecretKey, _ := base64.StdEncoding.DecodeString(c.SecretKey)
	return decodedSecretKey
}

func (c *BaseConfig) GetDecodedPublicKey(index int) [1184]byte {
	decodedPublicKey := [1184]byte{}
	decodedPubKey, _ := base64.StdEncoding.DecodeString(c.PublicKeys[index])
	copy(decodedPublicKey[:], decodedPubKey)
	return decodedPublicKey
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
