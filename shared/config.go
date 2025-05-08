package shared

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"pqgch/gake"
	"strings"
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

func (c *ClusterConfig) GetSecretKey() string {
	return c.SecretKey
}

func (c *ClusterConfig) GetNamesOrAddrs() []string {
	return c.Names
}

func (c *ClusterConfig) loadClusterKeys() []string {
	data, err := ioutil.ReadFile(c.PublicKeys)
	if err != nil {
		log.Fatalf("couldn't load cluster public-keys from %s: %v", c.PublicKeys, err)
	}
	var blob struct {
		PublicKeys []string `json:"publicKeys"`
	}
	if err := json.Unmarshal(data, &blob); err != nil {
		log.Fatalf("bad JSON in %s: %v", c.PublicKeys, err)
	}
	return blob.PublicKeys
}

func (c *ClusterConfig) GetKeys() []string {
	return c.loadClusterKeys()
}

func (c *ClusterConfig) GetDecodedPublicKey(i int) [gake.PkLen]byte {
	return getDecodedPublicKey(c.loadClusterKeys(), i)
}

func (c *ClusterConfig) GetDecodedPublicKeys() [][gake.PkLen]byte {
	raw := c.loadClusterKeys()
	out := make([][gake.PkLen]byte, len(raw))
	for i := range raw {
		out[i] = getDecodedPublicKey(raw, i)
	}
	return out
}

func (c *ClusterConfig) GetDecodedSecretKey() []byte {
	return getDecodedKey(c.SecretKey)
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

func (c *ServConfig) GetSecretKey() string {
	return c.SecretKey
}

func (c *ServConfig) GetNamesOrAddrs() []string {
	return c.ServAddrs
}

func (c *ServConfig) GetDecodedSecretKey() []byte {
	return getDecodedKey(c.SecretKey)
}

func (c *ServConfig) GetName() string {
	return c.ClusterConfig.GetName()
}

func (c *ServConfig) GetMessageType(msgType int) int {
	switch msgType {
	case XiRiCommitmentMsg:
		return LeaderXiRiCommitmentMsg
	case AkeAMsg:
		return LeaderAkeAMsg
	case AkeBMsg:
		return LeaderAkeBMsg
	default:
		return msgType
	}
}

type ClusterConfig struct {
	Names      []string `json:"names"`
	Index      int      `json:"index"`
	PublicKeys string   `json:"publicKeys"`
	SecretKey  string   `json:"secretKey"`
}

type UserConfig struct {
	ClusterConfig `json:"clusterConfig"`
	LeadAddr      string `json:"leadAddr"`
}

type ServConfig struct {
	ClusterConfig `json:"clusterConfig"`
	Index         int      `json:"index"`
	ServAddrs     []string `json:"servers"`
	SecretKey     string   `json:"secretKey"`
	KeyLeft       string   `json:"keyLeft"`
	KeyRight      string   `json:"keyRight"`
}

func (c *ServConfig) IsLeftQKD() bool {
	return strings.HasPrefix(strings.ToLower(c.KeyLeft), "path ") ||
		strings.HasPrefix(strings.ToLower(c.KeyLeft), "url ")
}

func (c *ServConfig) IsRightQKD() bool {
	return strings.HasPrefix(strings.ToLower(c.KeyRight), "path ") ||
		strings.HasPrefix(strings.ToLower(c.KeyRight), "url ")
}

func (c *ServConfig) GetLeftKey() string {
	if c.IsLeftQKD() {
		return strings.TrimSpace(c.KeyLeft[5:])
	}
	return c.KeyLeft
}

func (c *ServConfig) GetRightKey() string {
	if c.IsRightQKD() {
		return strings.TrimSpace(c.KeyRight[5:])
	}
	return c.KeyRight
}

func (c *ServConfig) GetDecodedLeftKeyPublic() [gake.PkLen]byte {
	return decodePublicKey(c.GetLeftKey())
}

func (c *ServConfig) GetDecodedRightKeyPublic() [gake.PkLen]byte {
	return decodePublicKey(c.GetRightKey())
}

func decodePublicKey(key string) [gake.PkLen]byte {
	var decoded [gake.PkLen]byte
	raw := getDecodedKey(key)
	copy(decoded[:], raw)
	return decoded
}

func (c *ServConfig) GetDecodedLeftKeyQKD() ([gake.SsLen]byte, error) {
	return openAndDecodeQKDKey(c.GetLeftKey())
}
func (c *ServConfig) GetDecodedRightKeyQKD() ([gake.SsLen]byte, error) {
	return openAndDecodeQKDKey(c.GetRightKey())
}

func openAndDecodeQKDKey(filePath string) ([gake.SsLen]byte, error) {
	var key [gake.SsLen]byte

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return key, err
	}
	trimmed := strings.TrimSpace(string(data))
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return key, err
	}
	if len(decoded) < gake.SsLen {
		return key, errors.New("[ERROR]cluster key file is too short")
	}
	copy(key[:], decoded)
	return key, nil
}

func GetUserConfig(path string) UserConfig {
	config := UserConfig{}

	configFile, err := os.Open(path)
	if err != nil {
		log.Fatalf("Error opening config file at %s: %v", path, err)
	}
	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
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

	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}
	return config
}

func getDecodedKey(secretKey string) []byte {
	decodedSecretKey, _ := base64.StdEncoding.DecodeString(secretKey)
	return decodedSecretKey
}

func getDecodedPublicKey(publicKeys []string, index int) [gake.PkLen]byte {
	decodedPublicKey := [gake.PkLen]byte{}
	decodedPubKey, _ := base64.StdEncoding.DecodeString(publicKeys[index])
	copy(decodedPublicKey[:], decodedPubKey)
	return decodedPublicKey
}

func (c *ServConfig) GetCurrentServer() string {
	return c.ServAddrs[c.Index]
}
