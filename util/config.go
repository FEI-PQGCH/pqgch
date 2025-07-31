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
	raw := openAndDecodeKey(c.SecretKey, gake.SkLen)
	fmt.Printf("Loaded cluster secret key from %s\n", c.SecretKey)
	return raw
}

func (c *ClusterConfig) IsClusterQKDPath() bool {
	return strings.HasPrefix(strings.ToLower(c.Crypto), "path ")
}

func (c *ClusterConfig) ClusterQKDPath() string {
	return strings.TrimSpace(c.Crypto[5:])
}

func (c *ClusterConfig) ClusterKey() ([gake.SsLen]byte, error) {
	var key [gake.SsLen]byte

	data, err := os.ReadFile(c.ClusterQKDPath())
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

func (c *ClusterConfig) Name() string {
	return c.Names[c.Index]
}

func (c *ClusterConfig) RightIndex() int {
	return (c.Index + 1) % len(c.Names)
}

func (c *ClusterConfig) IsClusterQKDUrl() bool {
	return strings.HasPrefix(strings.ToLower(c.Crypto), "url ")
}

func (c *ClusterConfig) ClusterQKDUrl() string {
	return strings.TrimSpace(c.Crypto[4:])
}

func (c *LeaderConfig) GetSecretKey() []byte {
	raw := openAndDecodeKey(c.SecretKey, gake.SkLen)
	fmt.Printf("Loaded cluster secret key from %s\n", c.SecretKey)
	fmt.Printf("Cluster secret key: %x\n", raw)
	return raw
}

func (c *LeaderConfig) RightIndex() int {
	return (c.Index + 1) % len(c.Addrs)
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

func (c *LeaderConfig) LeftQKDUrl() string {
	return strings.TrimSpace(c.Left[4:])
}

func (c *LeaderConfig) RightQKDUrl() string {
	return strings.TrimSpace(c.Right[4:])
}

func (c *LeaderConfig) LeftPublicKey() [gake.PkLen]byte {
	raw := openAndDecodeKey(c.Left, gake.PkLen)
	var out [gake.PkLen]byte
	copy(out[:], raw)
	return out
}

func (c *LeaderConfig) RightPublicKey() [gake.PkLen]byte {
	raw := openAndDecodeKey(c.Right, gake.PkLen)
	var out [gake.PkLen]byte
	copy(out[:], raw)
	return out
}

func (c *LeaderConfig) LeftQKDKey() [gake.SsLen]byte {
	raw := openAndDecodeKey(strings.TrimSpace(c.Left[5:]), gake.SsLen)
	var out [gake.SsLen]byte
	copy(out[:], raw)
	return out
}

func (c *LeaderConfig) RightQKDKey() [gake.SsLen]byte {
	raw := openAndDecodeKey(strings.TrimSpace(c.Right[5:]), gake.SsLen)
	var out [gake.SsLen]byte
	copy(out[:], raw)
	return out
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
	raw, err = hex.DecodeString(blob.Key)
	if err == nil {
		return raw, nil
	}
	return nil, fmt.Errorf("key in %q is neither valid base64 nor hex", path)
}

func openAndDecodeKey(path string, expectLen int) []byte {
	raw, err := loadJSONKey(path)
	if err != nil {
		fmt.Printf("Error loading key from %s: %v\n", path, err)
		return nil
	}
	if len(raw) != expectLen {
		fmt.Printf("Key length mismatch for %s: expected %d, got %d\n", path, expectLen, len(raw))
		return nil
	}
	return raw
}

func decodePublicKey(key string) [gake.PkLen]byte {
	var decoded [gake.PkLen]byte
	raw, _ := base64.StdEncoding.DecodeString(key)
	copy(decoded[:], raw)
	return decoded
}
