package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"pqgch/gake"
	"strings"
)

type BaseConfig struct {
	Server    string         `json:"server"`
	Name      string         `json:"name"`
	ClusterID int            `json:"clusterID"`
	Cluster   *ClusterConfig `json:"cluster,omitempty"`
	Leader    *LeaderConfig  `json:"leaders,omitempty"`
}

type ClusterConfig struct {
	NMembers   int    `json:"nMembers"`
	MemberID   int    `json:"memberID"`
	PublicKeys string `json:"publicKeys,omitempty"`
	SecretKey  string `json:"secretKey,omitempty"`
	Crypto     string `json:"crypto,omitempty"`
}

type LeaderConfig struct {
	NClusters   int    `json:"nClusters"`
	LeftCrypto  string `json:"leftCrypto"`
	RightCrypto string `json:"rightCrypto"`
	SecretKey   string `json:"secretKey"`
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

func (c *BaseConfig) HasCluster() bool {
	return c.Cluster != nil
}

func (c *BaseConfig) RightClusterID() int {
	return (c.ClusterID + 1) % c.Leader.NClusters
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

func (c *ClusterConfig) ClusterQKDKeyFromFile() ([2 * gake.SsLen]byte, error) {
	var key [2 * gake.SsLen]byte
	raw := openAndDecodeKey(strings.TrimSpace(c.Crypto[5:]), 2*gake.SsLen)
	copy(key[:], raw)
	return key, nil
}

func (c *ClusterConfig) RightMemberID() int {
	return (c.MemberID + 1) % c.NMembers
}

func (c *ClusterConfig) HasQKDUrl() bool {
	if c == nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(c.Crypto), "url ")
}

func (c *ClusterConfig) QKDUrl() string {
	return strings.TrimSpace(c.Crypto[4:])
}

func (c *LeaderConfig) GetSecretKey() []byte {
	raw := openAndDecodeKey(c.SecretKey, gake.SkLen)
	return raw
}

func (c *LeaderConfig) HasLeftQKDUrl() bool {
	return strings.HasPrefix(strings.ToLower(c.LeftCrypto), "url ")
}

func (c *LeaderConfig) HasLeftQKDPath() bool {
	return strings.HasPrefix(strings.ToLower(c.LeftCrypto), "path ")
}

func (c *LeaderConfig) HasRightQKDUrl() bool {
	return strings.HasPrefix(strings.ToLower(c.RightCrypto), "url ")
}

func (c *LeaderConfig) HasRightQKDPath() bool {
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
