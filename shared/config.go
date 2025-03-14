package shared

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"pqgch-client/gake"
)

type ConfigAccessor interface {
	GetIndex() int
	GetKeys() []string
	GetSecretKey() string
	GetNamesOrAddrs() []string
	GetDecodedSecretKey() []byte
	GetDecodedPublicKey(int) [gake.PkLen]byte
	GetDecodedPublicKeys() [][gake.PkLen]byte
	GetName() string
	GetMessageType(int) int
}

func (c *ClusterConfig) GetIndex() int {
	return c.Index
}

func (c *ClusterConfig) GetKeys() []string {
	return c.PublicKeys
}

func (c *ClusterConfig) GetSecretKey() string {
	return c.SecretKey
}

func (c *ClusterConfig) GetNamesOrAddrs() []string {
	return c.Names
}

func (c *ClusterConfig) GetDecodedPublicKey(index int) [gake.PkLen]byte {
	return getDecodedPublicKey(c.PublicKeys, index)
}

func (c *ClusterConfig) GetDecodedPublicKeys() [][gake.PkLen]byte {
	keys := make([][gake.PkLen]byte, len(c.GetNamesOrAddrs()))
	for i := 0; i < len(c.GetNamesOrAddrs()); i++ {
		keys[i] = c.GetDecodedPublicKey(i)
	}

	return keys
}

func (c *ClusterConfig) GetDecodedSecretKey() []byte {
	return getDecodedSecretKey(c.SecretKey)
}

func (c *ClusterConfig) GetName() string {
	return c.Names[c.Index]
}

func (c *ClusterConfig) GetMessageType(msgType int) int {
	return msgType
}

func (c *ServConfig) GetIndex() int {
	return c.Index
}

func (c *ServConfig) GetKeys() []string {
	return c.PublicKeys
}

func (c *ServConfig) GetSecretKey() string {
	return c.SecretKey
}

func (c *ServConfig) GetNamesOrAddrs() []string {
	return c.ServAddrs
}

func (c *ServConfig) GetDecodedPublicKey(index int) [gake.PkLen]byte {
	return getDecodedPublicKey(c.PublicKeys, index)
}

func (c *ServConfig) GetDecodedPublicKeys() [][gake.PkLen]byte {
	keys := make([][gake.PkLen]byte, len(c.GetNamesOrAddrs()))
	for i := 0; i < len(c.GetNamesOrAddrs()); i++ {
		keys[i] = c.GetDecodedPublicKey(i)
	}

	return keys
}

func (c *ServConfig) GetDecodedSecretKey() []byte {
	return getDecodedSecretKey(c.SecretKey)
}

func (c *ServConfig) GetName() string {
	return c.ClusterConfig.Names[c.ClusterConfig.Index]
}

func (c *ServConfig) GetMessageType(msgType int) int {
	switch msgType {
	case XiMsg:
		return LeaderXiMsg
	case AkeAMsg:
		return LeaderAkeAMsg
	case AkeBMsg:
		return LeaderAkeBMsg
	default:
		return msgType
	}
}

type ClusterConfig struct {
	Names          []string `json:"names"`
	Index          int      `json:"index"`
	PublicKeys     []string `json:"publicKeys"`
	SecretKey      string   `json:"secretKey"`
	ClusterKeyFile string   `json:"clusterKeyFile"`
}

type UserConfig struct {
	ClusterConfig `json:"clusterConfig"`
	LeadAddr      string `json:"leadAddr"`
}

type ServConfig struct {
	ClusterConfig `json:"clusterConfig"`
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

func getDecodedSecretKey(secretKey string) []byte {
	decodedSecretKey, _ := base64.StdEncoding.DecodeString(secretKey)
	return decodedSecretKey
}

func getDecodedPublicKey(publicKeys []string, index int) [gake.PkLen]byte {
	decodedPublicKey := [gake.PkLen]byte{}
	decodedPubKey, _ := base64.StdEncoding.DecodeString(publicKeys[index])
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

func (c *ServConfig) GetRightNeighbor() string {
	if c.Index == len(c.ServAddrs)-1 {
		return c.ServAddrs[0]
	}
	return c.ServAddrs[c.Index+1]
}

func (c *ServConfig) GetCurrentServer() string {
	return c.ServAddrs[c.Index]
}
