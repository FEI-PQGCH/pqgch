package util

import (
	"encoding/base64"
	"encoding/json"
	"errors"
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

func exactlyOne(bools ...bool) bool {
	count := 0
	for _, b := range bools {
		if b {
			count++
			if count > 1 {
				return false
			}
		}
	}
	return count == 1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (c *BaseConfig) Validate() error {
	var errs []string

	if strings.TrimSpace(c.Server) == "" {
		errs = append(errs, "missing required field: server")
	}
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, "missing required field: name")
	}
	if c.ClusterID < 0 {
		errs = append(errs, "clusterID must be ≥ 0")
	}

	if c.Cluster != nil {
		if err := c.Cluster.Validate(); err != nil {
			errs = append(errs, prefixErrLines("cluster", err)...)
		}
	}
	if c.Leader != nil {
		if err := c.Leader.Validate(); err != nil {
			errs = append(errs, prefixErrLines("leaders", err)...)
		}
	}

	if len(errs) > 0 {
		return logError(errs)
	}
	return nil
}

func (c *ClusterConfig) Validate() error {
	var errs []string

	if c.NMembers <= 0 {
		errs = append(errs, "nMembers must be > 0")
	}

	hasCrypto := strings.TrimSpace(c.Crypto) != ""
	hasPK := strings.TrimSpace(c.PublicKeys) != ""
	hasSK := strings.TrimSpace(c.SecretKey) != ""
	hasPair := hasPK && hasSK

	if !exactlyOne(hasCrypto, hasPair) {
		if !hasCrypto && !hasPair {
			errs = append(errs, "must provide either crypto OR (publicKeys + secretKey)")
		} else if hasCrypto && (hasPK != hasSK) {
			errs = append(errs, "when crypto is set, publicKeys and secretKey must be absent (not just one of them)")
		} else {
			errs = append(errs, "use either crypto OR (publicKeys + secretKey), not both")
		}
	}

	if hasCrypto {
		crypto := strings.TrimSpace(c.Crypto)
		low := strings.ToLower(crypto)
		switch {
		case strings.HasPrefix(low, "path "):
			p := strings.TrimSpace(crypto[5:])
			if err := validateJSONKeyLen(p, 2*gake.SsLen); err != nil {
				errs = append(errs, fmt.Sprintf("QKD key at path is invalid: %v", err))
			}
		case strings.HasPrefix(low, "url "):
			errs = append(errs, "crypto must not be URL; only 'path <file>' is allowed for clusters")
		default:
			errs = append(errs, "crypto must start with 'path ' and point to a JSON-wrapped QKD key")
		}
	}

	if !hasCrypto {
		if !hasPK || !hasSK {
			errs = append(errs, "classical mode requires both: publicKeys and secretKey")
		} else {
			if err := validatePublicKeysFile(c.PublicKeys, c.NMembers); err != nil {
				errs = append(errs, fmt.Sprintf("publicKeys file invalid: %v", err))
			}
			if err := validateJSONKeyLen(c.SecretKey, gake.SkLen); err != nil {
				errs = append(errs, fmt.Sprintf("secretKey file invalid: %v", err))
			}
		}
	}

	if len(errs) > 0 {
		return logError(errs)
	}
	return nil
}

func (c *LeaderConfig) Validate() error {
	var errs []string

	if c.NClusters <= 0 {
		errs = append(errs, "nClusters must be > 0")
	}
	if strings.TrimSpace(c.SecretKey) == "" {
		errs = append(errs, "missing required field: secretKey")
	} else if err := validateJSONKeyLen(c.SecretKey, gake.SkLen); err != nil {
		errs = append(errs, fmt.Sprintf("secretKey file invalid: %v", err))
	}

	checkPKFile := func(label, v string) {
		vtrim := strings.TrimSpace(v)
		if vtrim == "" {
			errs = append(errs, fmt.Sprintf("missing required field: %s (must be PK file path)", label))
			return
		}
		low := strings.ToLower(vtrim)
		if strings.HasPrefix(low, "url ") || strings.HasPrefix(low, "path ") {
			errs = append(errs, fmt.Sprintf("%s must be a PK file path (no 'url ' or 'path ' allowed for leaders)", label))
			return
		}
		if err := validateJSONKeyLen(vtrim, gake.PkLen); err != nil {
			errs = append(errs, fmt.Sprintf("%s public key file invalid: %v", label, err))
		}
	}

	checkPKFile("leftCrypto", c.LeftCrypto)
	checkPKFile("rightCrypto", c.RightCrypto)

	if len(errs) > 0 {
		return logError(errs)
	}
	return nil
}

func logError(lines []string) error {
	seen := map[string]struct{}{}
	var buf strings.Builder
	buf.WriteString("Invalid configuration:\n")
	for _, l := range lines {
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		buf.WriteString("  • ")
		buf.WriteString(l)
		buf.WriteByte('\n')
	}
	return errors.New(strings.TrimRight(buf.String(), "\n"))
}

func prefixErrLines(prefix string, err error) []string {
	if err == nil {
		return nil
	}
	out := []string{}
	for _, l := range strings.Split(err.Error(), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "Invalid configuration:") {
			continue
		}
		out = append(out, fmt.Sprintf("%s: %s", prefix, strings.TrimPrefix(l, "• ")))
	}
	return out
}

func validateJSONKeyLen(path string, expectLen int) error {
	raw, err := loadJSONKey(path)
	if err != nil {
		return fmt.Errorf("cannot load key %q: %w", path, err)
	}
	if len(raw) != expectLen {
		return fmt.Errorf("key %q has wrong length: expected %d, got %d", path, expectLen, len(raw))
	}
	return nil
}

func validatePublicKeysFile(path string, wantAtLeast int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read publicKeys file %q: %w", path, err)
	}
	var blob struct {
		PublicKeys []string `json:"publicKeys"`
	}
	if err := json.Unmarshal(data, &blob); err != nil {
		return fmt.Errorf("invalid JSON in %q: %w", path, err)
	}
	if len(blob.PublicKeys) == 0 {
		return fmt.Errorf("publicKeys array is empty in %q", path)
	}
	if wantAtLeast > 0 && len(blob.PublicKeys) < wantAtLeast {
		return fmt.Errorf("publicKeys count (%d) is less than nMembers (%d)", len(blob.PublicKeys), wantAtLeast)
	}
	for i, s := range blob.PublicKeys {
		raw, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return fmt.Errorf("publicKeys[%d] is not valid base64", i)
		}
		if len(raw) != gake.PkLen {
			return fmt.Errorf("publicKeys[%d] has wrong length: expected %d, got %d", i, gake.PkLen, len(raw))
		}
	}
	return nil
}

func GetConfig[T any](path string) (T, error) {
	var config T

	configFile, err := os.Open(path)
	if err != nil {
		return config, fmt.Errorf("cannot open config %q: %w", path, err)
	}
	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return config, fmt.Errorf("invalid JSON in %q: %w", path, err)
	}

	if validator, ok := any(&config).(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			return config, err
		}
	}
	return config, nil
}

func (c *BaseConfig) HasCluster() bool {
	return c.Cluster != nil
}

func (c *BaseConfig) RightClusterID() int {
	return (c.ClusterID + 1) % c.Leader.NClusters
}

func (c *BaseConfig) GetMemberID() int {
	memberID := 0
	if c.HasCluster() {
		memberID = c.Cluster.MemberID
	}
	return memberID
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
