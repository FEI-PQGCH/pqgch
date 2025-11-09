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
	ClusterID *int           `json:"clusterID"`
	Cluster   *ClusterConfig `json:"cluster,omitempty"`
	Leader    *LeaderConfig  `json:"leaders,omitempty"`
}

type ClusterConfig struct {
	NMembers   *int   `json:"nMembers"`
	MemberID   *int   `json:"memberID"`
	PublicKeys string `json:"publicKeys,omitempty"`
	SecretKey  string `json:"secretKey,omitempty"`
	Crypto     string `json:"crypto,omitempty"`
}

type LeaderConfig struct {
	NClusters   *int   `json:"nClusters"`
	LeftCrypto  string `json:"leftCrypto"`
	RightCrypto string `json:"rightCrypto"`
	SecretKey   string `json:"secretKey"`
}

const (
	pathQKDPrefix = "path "
	urlQKDPrefix  = "url "
)

func (c *BaseConfig) validate(isLeader bool) []string {
	var errs []string

	if strings.TrimSpace(c.Server) == "" {
		errs = append(errs, "missing required field: server")
	}
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, "missing required field: name")
	}
	if c.ClusterID == nil || *c.ClusterID < 0 {
		errs = append(errs, "clusterID must be set and >= 0")
	}
	if isLeader && c.Leader == nil {
		errs = append(errs, "missing required field: leaders")
	}
	if !isLeader && c.Cluster == nil {
		errs = append(errs, "missing required field: cluster")
	}

	if len(errs) > 0 {
		return errs
	}

	if c.Cluster != nil {
		if err := c.Cluster.validate(); err != nil {
			errs = append(errs, err...)
		}
	}
	if c.Leader != nil {
		if err := c.Leader.validate(); err != nil {
			errs = append(errs, err...)
		}
	}

	return errs
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

func isEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

func (c *ClusterConfig) validate() []string {
	var errs []string

	if c.NMembers == nil || *c.NMembers <= 0 {
		errs = append(errs, "nMembers must be set and > 0")
	}
	if c.MemberID == nil || *c.MemberID < 0 {
		errs = append(errs, "memberID must be set and >= 0")
	}

	if len(errs) > 0 {
		return errs
	}

	hasCrypto := !isEmpty(c.Crypto)
	hasPK := !isEmpty(c.PublicKeys)
	hasSK := !isEmpty(c.SecretKey)
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
		case strings.HasPrefix(low, pathQKDPrefix):
			p := strings.TrimSpace(crypto[5:])
			if err := validateJSONKeyLen(p, 2*gake.SsLen); err != nil {
				errs = append(errs, fmt.Sprintf("QKD key at path is invalid: %v", err))
			}
		case strings.HasPrefix(low, urlQKDPrefix):
		default:
			errs = append(errs, "crypto must start with 'path ' and point to a JSON-wrapped QKD key or with 'url ' and contain an URL to the ETSI server")
		}
	}

	if !hasCrypto {
		if !hasPK || !hasSK {
			errs = append(errs, "Kyber-GAKE mode requires both: publicKeys and secretKey")
		} else {
			if err := validatePublicKeysFile(c.PublicKeys, *c.NMembers); err != nil {
				errs = append(errs, fmt.Sprintf("publicKeys file invalid: %v", err))
			}
			if err := validateJSONKeyLen(c.SecretKey, gake.SkLen); err != nil {
				errs = append(errs, fmt.Sprintf("secretKey file invalid: %v", err))
			}
		}
	}

	return errs
}
func isQKD(value string) bool {
	low := strings.ToLower(value)
	if strings.HasPrefix(low, pathQKDPrefix) || strings.HasPrefix(low, urlQKDPrefix) {
		return true
	}
	return false
}

func validateCrypto(key, value string) []string {
	var errs []string

	low := strings.ToLower(value)
	if strings.HasPrefix(low, pathQKDPrefix) {
		p := strings.TrimSpace(value[5:])
		if err := validateJSONKeyLen(p, gake.SsLen); err != nil {
			errs = append(errs, fmt.Sprintf("QKD key at path is invalid: %v", err))
		}
		return errs
	}
	if strings.HasPrefix(low, urlQKDPrefix) {
		return errs
	}
	if err := validateJSONKeyLen(value, gake.PkLen); err != nil {
		errs = append(errs, fmt.Sprintf("%s public key file invalid: %v", key, err))
	}

	return errs
}

func validateSK(sk string) []string {
	var errs []string

	if isEmpty(sk) {
		errs = append(errs, "missing required field: secretKey")
	} else if err := validateJSONKeyLen(sk, gake.SkLen); err != nil {
		errs = append(errs, fmt.Sprintf("secretKey file invalid: %v", err))
	}

	return errs
}

func missingCryptoMsg(key string) string {
	return fmt.Sprintf("missing required field: %s (must be PK file path, QKD secret key file path or an URL to the ETSI server)", key)
}

func (c *LeaderConfig) validate() []string {
	var errs []string

	if c.NClusters == nil || *c.NClusters <= 0 {
		errs = append(errs, "nClusters must be set and > 0")
	}
	if isEmpty(c.LeftCrypto) {
		errs = append(errs, missingCryptoMsg("leftCrypto"))
	}
	if isEmpty(c.RightCrypto) {
		errs = append(errs, missingCryptoMsg("rightCrypto"))
	}

	hasBothCrypto := !isEmpty(c.LeftCrypto) && !isEmpty(c.RightCrypto)
	if hasBothCrypto {
		errs = append(errs, validateCrypto("leftCrypto", c.LeftCrypto)...)
		errs = append(errs, validateCrypto("rightCrypto", c.RightCrypto)...)

		atleastOneNotQKD := !isQKD(c.LeftCrypto) || !isQKD(c.RightCrypto)
		if atleastOneNotQKD {
			errs = append(errs, validateSK(c.SecretKey)...)
		}
	}

	return errs
}

func formatErrors(lines []string) error {
	var buf strings.Builder
	buf.WriteString("invalid configuration:\n")
	for _, l := range lines {
		buf.WriteString("- " + l + "\n")
	}
	return errors.New(strings.TrimRight(buf.String(), "\n"))
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

func validatePublicKeysFile(path string, n int) error {
	_, err := getPublicKeys(path, n)
	return err
}

func GetConfig(path string, isLeader bool) (BaseConfig, error) {
	var config BaseConfig

	configFile, err := os.Open(path)
	if err != nil {
		return config, fmt.Errorf("cannot open config %q: %w", path, err)
	}
	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return config, fmt.Errorf("invalid JSON in %q: %w", path, err)
	}

	errs := config.validate(isLeader)
	if len(errs) > 0 {
		return config, formatErrors(errs)
	}

	return config, nil
}

func (c *BaseConfig) HasCluster() bool {
	return c.Cluster != nil
}

func (c *BaseConfig) RightClusterID() int {
	return (*c.ClusterID + 1) % *c.Leader.NClusters
}

func (c *BaseConfig) GetMemberID() int {
	if c.HasCluster() {
		return *c.Cluster.MemberID
	}
	return 0
}

func (c *ClusterConfig) GetPublicKeys() [][gake.PkLen]byte {
	pks, err := getPublicKeys(c.PublicKeys, *c.NMembers)
	if err != nil {
		ExitWithMsg(fmt.Sprintf("failed to load cluster PKs from file %s, error: %v", c.PublicKeys, err))
	}
	return pks
}

func (c *ClusterConfig) GetSecretKey() []byte {
	raw := openAndDecodeKey(c.SecretKey, gake.SkLen)
	return raw
}

func (c *ClusterConfig) IsClusterQKDPath() bool {
	return strings.HasPrefix(strings.ToLower(c.Crypto), pathQKDPrefix)
}

func (c *ClusterConfig) ClusterQKDKeyFromFile() ([2 * gake.SsLen]byte, error) {
	var key [2 * gake.SsLen]byte
	raw := openAndDecodeKey(strings.TrimSpace(c.Crypto[5:]), 2*gake.SsLen)
	copy(key[:], raw)
	return key, nil
}

func (c *ClusterConfig) RightMemberID() int {
	return (*c.MemberID + 1) % *c.NMembers
}

func (c *ClusterConfig) HasQKDUrl() bool {
	if c == nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(c.Crypto), urlQKDPrefix)
}

func (c *ClusterConfig) QKDUrl() string {
	return strings.TrimSpace(c.Crypto[4:])
}

func (c *LeaderConfig) GetSecretKey() []byte {
	raw := openAndDecodeKey(c.SecretKey, gake.SkLen)
	return raw
}

func (c *LeaderConfig) HasLeftQKDUrl() bool {
	return strings.HasPrefix(strings.ToLower(c.LeftCrypto), urlQKDPrefix)
}

func (c *LeaderConfig) HasLeftQKDPath() bool {
	return strings.HasPrefix(strings.ToLower(c.LeftCrypto), pathQKDPrefix)
}

func (c *LeaderConfig) HasRightQKDUrl() bool {
	return strings.HasPrefix(strings.ToLower(c.RightCrypto), urlQKDPrefix)
}

func (c *LeaderConfig) HasRightQKDPath() bool {
	return strings.HasPrefix(strings.ToLower(c.RightCrypto), pathQKDPrefix)
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

func getPublicKeys(path string, n int) ([][gake.PkLen]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read publicKeys file %q: %w", path, err)
	}
	var blob struct {
		PublicKeys []string `json:"publicKeys"`
	}
	if err := json.Unmarshal(data, &blob); err != nil {
		return nil, fmt.Errorf("invalid JSON in %q: %w", path, err)
	}
	if len(blob.PublicKeys) == 0 {
		return nil, fmt.Errorf("publicKeys array is empty in %q", path)
	}
	if len(blob.PublicKeys) != n {
		return nil, fmt.Errorf("publicKeys count (%d) is not equal to nMembers (%d)", len(blob.PublicKeys), n)
	}

	out := make([][gake.PkLen]byte, len(blob.PublicKeys))
	for i, s := range blob.PublicKeys {
		raw, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("publicKeys[%d] is not valid base64", i)
		}
		if len(raw) != gake.PkLen {
			return nil, fmt.Errorf("publicKeys[%d] has wrong length: expected %d, got %d", i, gake.PkLen, len(raw))
		}
		copy(out[i][:], raw[:])
	}
	return out, nil
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
