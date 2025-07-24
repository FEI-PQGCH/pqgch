package util

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"pqgch/gake"
	"strings"
)

type ClusterConfig struct {
	Names      []string `json:"names"`
	Index      int      `json:"index"`
	PublicKeys string   `json:"publicKeys,omitempty"`
	SecretKey  string   `json:"secretKey,omitempty"`
	Crypto     string   `json:"crypto,omitempty"`
}

type UserConfig struct {
	ClusterConfig `json:"clusterConfig"`
	LeadAddr      string `json:"leadAddr"`
}

type LeaderConfig struct {
	ClusterConfig `json:"clusterConfig"`
	Index         int      `json:"index"`
	Addrs         []string `json:"servers"`
	SecretKey     string   `json:"secretKey"`
	Left          string   `json:"leftCrypto"`
	Right         string   `json:"rightCrypto"`
}

func GetConfig[T any](path string) (T, error) {
	var config T

	configFile, err := os.Open(path)
	if err != nil {
		return config, fmt.Errorf("error opening config file at %s: %v", path, err)
	}
	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return config, fmt.Errorf("error parsing config file: %v", err)
	}

	return config, nil
}

func (c *ClusterConfig) GetPublicKeys() [][gake.PkLen]byte {
	data, err := os.ReadFile(c.PublicKeys)
	if err != nil {
		// TODO: return fmt.Errorf("couldn't load cluster public-keys from %s: %v", c.PublicKeys, err)
		return nil
	}

	var blob struct {
		PublicKeys []string `json:"publicKeys"`
	}

	if err := json.Unmarshal(data, &blob); err != nil {
		// TODO: return fmt.Errorf("bad JSON in %s: %v", c.PublicKeys, err)
		return nil
	}

	out := make([][gake.PkLen]byte, len(blob.PublicKeys))
	for i := range blob.PublicKeys {
		out[i] = decodePublicKey(blob.PublicKeys[i])
	}
	return out
}

func (c *ClusterConfig) GetSecretKey() []byte {
	key, _ := base64.StdEncoding.DecodeString(c.SecretKey)
	return key
}

func (c *ClusterConfig) IsClusterQKDPath() bool {
	return strings.HasPrefix(strings.ToLower(c.Crypto), "path ")
}

func (c *ClusterConfig) GetClusterQKDPath() string {
	return strings.TrimSpace(c.Crypto[5:])
}

func (c *ClusterConfig) GetClusterKey() ([gake.SsLen]byte, error) {
	var key [gake.SsLen]byte

	data, err := os.ReadFile(c.GetClusterQKDPath())
	if err != nil {
		return key, err
	}
	trimmed := strings.TrimSpace(string(data))
	dec, err := hex.DecodeString(trimmed)
	if err != nil {
		return key, err
	}
	if len(dec) != len(key) {
		return key, errors.New("cluster QKD key length mismatch")
	}
	copy(key[:], dec)
	return key, nil
}

func (c *ClusterConfig) GetName() string {
	return c.Names[c.Index]
}

func (c *ClusterConfig) GetMessageType(msgType int) int {
	return msgType
}

func (c *ClusterConfig) IsClusterQKDUrl() bool {
	return strings.HasPrefix(strings.ToLower(c.Crypto), "url ")
}

func (c *ClusterConfig) GetClusterQKDUrl() string {
	return strings.TrimSpace(c.Crypto[4:])
}

func (c *LeaderConfig) GetSecretKey() []byte {
	key, _ := base64.StdEncoding.DecodeString(c.SecretKey)
	return key
}

func (c *LeaderConfig) GetMessageType(msgType int) int {
	switch msgType {
	case XiRiCommitmentMsg:
		return LeaderXiRiCommitmentMsg
	case AkeOneMsg:
		return LeadAkeOneMsg
	case AkeTwoMsg:
		return LeadAkeTwoMsg
	default:
		return msgType
	}
}

func (c *LeaderConfig) IsLeftQKDUrl() bool {
	return strings.HasPrefix(strings.ToLower(c.Left), "url ")
}

func (c *LeaderConfig) IsLeftQKDPath() bool {
	return strings.HasPrefix(strings.ToLower(c.Left), "path ")
}

func (c *LeaderConfig) IsRightQKDUrl() bool {
	return strings.HasPrefix(strings.ToLower(c.Right), "url ")
}

func (c *LeaderConfig) IsRightQKDPath() bool {
	return strings.HasPrefix(strings.ToLower(c.Right), "path ")
}

func (c *LeaderConfig) GetLeftQKDURL() string {
	return strings.TrimSpace(c.Left[4:])
}

func (c *LeaderConfig) GetRightQKDURL() string {
	return strings.TrimSpace(c.Right[4:])
}

func (c *LeaderConfig) GetLeftPublicKey() [gake.PkLen]byte {
	return decodePublicKey(c.Left)
}

func (c *LeaderConfig) GetRightPublicKey() [gake.PkLen]byte {
	return decodePublicKey(c.Right)
}

func (c *LeaderConfig) GetLeftQKDKey() ([gake.SsLen]byte, error) {
	return openAndDecodeKey(strings.TrimSpace(c.Left[5:]))
}

func (c *LeaderConfig) GetRightQKDKey() ([gake.SsLen]byte, error) {
	return openAndDecodeKey(strings.TrimSpace(c.Right[5:]))
}

func openAndDecodeKey(filePath string) ([gake.SsLen]byte, error) {
	var key [gake.SsLen]byte

	data, err := os.ReadFile(filePath)
	if err != nil {
		return key, err
	}
	trimmed := strings.TrimSpace(string(data))
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return key, err
	}
	if len(decoded) != gake.SsLen {
		return key, errors.New("key length mismatch")
	}
	copy(key[:], decoded)
	return key, nil
}

func decodePublicKey(key string) [gake.PkLen]byte {
	var decoded [gake.PkLen]byte
	raw, _ := base64.StdEncoding.DecodeString(key)
	copy(decoded[:], raw)
	return decoded
}
