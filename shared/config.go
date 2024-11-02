package shared

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
)

type UserConfig struct {
	LeadAddr   string   `json:"leadAddr"`
	Names      []string `json:"names"`
	Index      int      `json:"index"`
	PublicKeys []string `json:"publicKeys"`
	SecretKey  string   `json:"secretKey"`
}

type ServConfig struct {
	Names     []string `json:"names"`
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

func (c *UserConfig) GetName() string {
	return c.Names[c.Index]
}

func (c *UserConfig) GetDecodedSecretKey() []byte {
	decodedSecretKey, _ := base64.StdEncoding.DecodeString(c.SecretKey)
	return decodedSecretKey
}

func (c *UserConfig) GetDecodedPublicKey(index int) [1184]byte {
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
