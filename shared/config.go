package shared

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
)

type ClusterConfig struct {
	Names      []string `json:"names"`
	Index      int      `json:"index"`
	PublicKeys []string `json:"publicKeys"`
	SecretKey  string   `json:"secretKey"`
}

type UserConfig struct {
	ClusterConfig
	LeadAddr string `json:"leadAddr"`
}

type ServConfig struct {
	ClusterConfig `json:"baseConfig"`
	Index         int      `json:"index"`
	ServAddrs     []string `json:"servers"`
	PublicKeys    []string `json:"publicKeys"`
	SecretKey     string   `json:"secretKey"`
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

func (c *ClusterConfig) GetName() string {
	return c.Names[c.Index]
}

func getDecodedSecretKey(secretKey string) []byte {
	decodedSecretKey, _ := base64.StdEncoding.DecodeString(secretKey)
	return decodedSecretKey
}

func (c *ClusterConfig) GetDecodedSecretKey() []byte {
	return getDecodedSecretKey(c.SecretKey)
}

func (c *ServConfig) GetDecodedSecretKey() []byte {
	return getDecodedSecretKey(c.SecretKey)
}

func getDecodedPublicKey(publicKeys []string, index int) [1184]byte {
	decodedPublicKey := [1184]byte{}
	decodedPubKey, _ := base64.StdEncoding.DecodeString(publicKeys[index])
	copy(decodedPublicKey[:], decodedPubKey)
	return decodedPublicKey
}

func (c *ServConfig) GetDecodedPublicKey(index int) [1184]byte {
	return getDecodedPublicKey(c.PublicKeys, index)
}

func (c *ClusterConfig) GetDecodedPublicKey(index int) [1184]byte {
	return getDecodedPublicKey(c.PublicKeys, index)
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

func (c *ServConfig) GetRightNeighbor() string {
	if c.Index == len(c.ServAddrs)-1 {
		return c.ServAddrs[0]
	}
	return c.ServAddrs[c.Index+1]
}

func (c *ServConfig) GetCurrentServer() string {
	return c.ServAddrs[c.Index]
}
