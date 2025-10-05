package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"pqgch/gake"
	"strings"
)

type ClusterConfig struct {
	ClusterID  int      `json:"clusterID"`
	MemberID   int      `json:"memberID,omitempty"`
	Names      []string `json:"names,omitempty"`
	PublicKeys string   `json:"publicKeys,omitempty"`
	SecretKey  string   `json:"secretKey,omitempty"`
	Crypto     string   `json:"crypto,omitempty"`
}

type MemberConfig struct {
	ClusterConfig ClusterConfig `json:"cluster"`
	Server        string        `json:"server"`
}

type LeaderConfig struct {
	ClusterConfig ClusterConfig `json:"cluster"`
	Server        string        `json:"server"`
	LeaderNames   []string      `json:"leaderNames"`
	SecretKey     string        `json:"secretKey"`
	LeftCrypto    string        `json:"leftCrypto"`
	RightCrypto   string        `json:"rightCrypto"`
}

func GetConfig[T any](path string) (T, error) {
	var config T

	configFile, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return config, err
	}

	return config, nil
}

func (c *ClusterConfig) GetPublicKeys() [][gake.PkLen]byte {
	data, err := os.ReadFile(c.PublicKeys)
	if err != nil {
		ExitWithMsg(fmt.Sprintf("couldn't load cluster public keys from %s: %v", c.PublicKeys, err))
	}

	var blob struct {
		PublicKeys []string `json:"publicKeys"`
	}

	if err := json.Unmarshal(data, &blob); err != nil {
		ExitWithMsg(fmt.Sprintf("bad JSON in %s: %v", c.PublicKeys, err))
	}

	out := make([][gake.PkLen]byte, len(blob.PublicKeys))
	for i := range blob.PublicKeys {
		out[i] = decodePublicKey(blob.PublicKeys[i])
	}
	return out
}

func (c *ClusterConfig) GetSecretKey() []byte {
	raw := openAndDecodeKey(c.SecretKey, gake.SkLen)
	return raw
}

func (c *ClusterConfig) IsClusterQKDPath() bool {
	return strings.HasPrefix(strings.ToLower(c.Crypto), "path ")
}

func (c *ClusterConfig) clusterQKDPath() string {
	return strings.TrimSpace(c.Crypto[5:])
}

func (c *ClusterConfig) ClusterQKDKeyFromFile() ([2 * gake.SsLen]byte, error) {
	var key [2 * gake.SsLen]byte
	raw := openAndDecodeKey(c.clusterQKDPath(), 2*gake.SsLen)
	copy(key[:], raw)
	return key, nil
}

func (c *ClusterConfig) Name() string {
	if c == nil {
		return ""
	}
	return c.Names[c.MemberID]
}

func (c *ClusterConfig) RightIndex() int {
	return (c.MemberID + 1) % len(c.Names)
}

func (c *ClusterConfig) IsClusterQKDUrl() bool {
	if c == nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(c.Crypto), "url ")
}

func (c *ClusterConfig) ClusterQKDUrl() string {
	return strings.TrimSpace(c.Crypto[4:])
}

func (c *ClusterConfig) GetIndex() int {
	if c == nil {
		return -1
	}
	return c.MemberID
}

func (c *ClusterConfig) HasCluster() bool {
	return len(c.Crypto) != 0 || len(c.PublicKeys) != 0 || len(c.SecretKey) != 0
}

func (c *LeaderConfig) GetSecretKey() []byte {
	raw := openAndDecodeKey(c.SecretKey, gake.SkLen)
	return raw
}

func (c *LeaderConfig) RightIndex() int {
	return (c.ClusterConfig.ClusterID + 1) % len(c.LeaderNames)
}

func (c *LeaderConfig) IsLeftQKDUrl() bool {
	return strings.HasPrefix(strings.ToLower(c.LeftCrypto), "url ")
}

func (c *LeaderConfig) IsLeftQKDPath() bool {
	return strings.HasPrefix(strings.ToLower(c.LeftCrypto), "path ")
}

func (c *LeaderConfig) IsRightQKDUrl() bool {
	return strings.HasPrefix(strings.ToLower(c.RightCrypto), "url ")
}

func (c *LeaderConfig) IsRightQKDPath() bool {
	return strings.HasPrefix(strings.ToLower(c.RightCrypto), "path ")
}

func (c *LeaderConfig) LeftQKDUrl() string {
	return strings.TrimSpace(c.LeftCrypto[4:])
}

func (c *LeaderConfig) RightQKDUrl() string {
	return strings.TrimSpace(c.RightCrypto[4:])
}

func (c *LeaderConfig) LeftPublicKey() [gake.PkLen]byte {
	raw := openAndDecodeKey(c.LeftCrypto, gake.PkLen)
	var out [gake.PkLen]byte
	copy(out[:], raw)
	return out
}

func (c *LeaderConfig) RightPublicKey() [gake.PkLen]byte {
	raw := openAndDecodeKey(c.RightCrypto, gake.PkLen)
	var out [gake.PkLen]byte
	copy(out[:], raw)
	return out
}

func (c *LeaderConfig) LeftQKDKey() [gake.SsLen]byte {
	raw := openAndDecodeKey(strings.TrimSpace(c.LeftCrypto[5:]), gake.SsLen)
	var out [gake.SsLen]byte
	copy(out[:], raw)
	return out
}

func (c *LeaderConfig) RightQKDKey() [gake.SsLen]byte {
	raw := openAndDecodeKey(strings.TrimSpace(c.RightCrypto[5:]), gake.SsLen)
	var out [gake.SsLen]byte
	copy(out[:], raw)
	return out
}

func (c *LeaderConfig) Name() string {
	if c == nil {
		return ""
	}
	return c.LeaderNames[c.ClusterConfig.ClusterID]
}

func openAndDecodeKey(path string, expectLen int) []byte {
	raw, err := loadJSONKey(path)
	if err != nil {
		ExitWithMsg(fmt.Sprintf("Error loading key from %s: %v\n", path, err))
	}
	if len(raw) != expectLen {
		ExitWithMsg(fmt.Sprintf("Key length mismatch for %s: expected %d, got %d\n", path, expectLen, len(raw)))
	}
	return raw
}

func loadJSONKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read key file %q: %w", path, err)
	}
	var blob struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &blob); err != nil {
		return nil, fmt.Errorf("invalid JSON in %q: %w", path, err)
	}
	raw, err := base64.StdEncoding.DecodeString(blob.Key)
	if err == nil {
		return raw, nil
	}
	return nil, fmt.Errorf("key in %q is invalid base64", path)
}

func decodePublicKey(key string) [gake.PkLen]byte {
	var decoded [gake.PkLen]byte
	raw, _ := base64.StdEncoding.DecodeString(key)
	copy(decoded[:], raw)
	return decoded
}
