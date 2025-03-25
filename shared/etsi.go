package shared

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"pqgch/gake"
)

// ETSIKeyResponse represents the JSON structure returned by the ETSI API.
type ETSIKeyResponse struct {
	Key string `json:"key"`
}

// GetKeyFromETSI calls the given ETSI API endpoint and returns a key of length gake.SsLen.
func GetKeyFromETSI(apiEndpoint string) ([gake.SsLen]byte, error) {
	var key [gake.SsLen]byte

	resp, err := http.Get(apiEndpoint)
	if err != nil {
		return key, fmt.Errorf("failed to call ETSI API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return key, fmt.Errorf("ETSI API returned non-OK status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return key, fmt.Errorf("failed to read ETSI API response: %v", err)
	}

	var etsirep ETSIKeyResponse
	if err := json.Unmarshal(body, &etsirep); err != nil {
		return key, fmt.Errorf("failed to parse ETSI API response: %v", err)
	}

	decodedKey, err := base64.StdEncoding.DecodeString(etsirep.Key)
	if err != nil {
		return key, fmt.Errorf("failed to decode ETSI API key: %v", err)
	}
	if len(decodedKey) < gake.SsLen {
		return key, fmt.Errorf("received key is too short")
	}
	copy(key[:], decodedKey)
	return key, nil
}
