package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"pqgch/gake"
	"strconv"
)

type Key struct {
	KeyID string `json:"key_ID"`
	Key   string `json:"key"`
}

type KeyContainer struct {
	Keys []Key `json:"keys"`
}

// EXAMPLE USAGE
//
// TODO: use this instead of loading the keys from the file.
// key_id should be sent to the neighbor, so he can get the same key.
//
// key, key_id, err := util.GetKey("http://localhost:8080/etsi/", "dummy_id")
// if err != nil {
// 	log.Fatalln(err.Error())
// }
// fmt.Println(key, key_id)

// key, key_id, err = util.GetKeyWithID("http://localhost:8080/etsi/", "dummy_id", key_id)
// if err != nil {
// 	log.Fatalln(err.Error())
// }
// fmt.Println(key, key_id)

// Make a request for a SINGLE key to the ETSI QKD API.
// The length is calculated from gake.SsLen.
func GetKey(endpoint, saeID string) (string, string, error) {
	resp, err := http.Get(endpoint + saeID + "/enc_keys?number=1&size=" + strconv.Itoa(gake.SsLen*8))
	if err != nil {
		return "", "", fmt.Errorf("failed to call ETSI API: %v", err)
	}

	key, id, err := parseResponse(resp)

	return key, id, err
}

// Make a request for the key with keyID to the ETSI QKD API.
func GetKeyWithID(endpoint, saeID, keyID string) (string, string, error) {
	resp, err := http.Get(endpoint + saeID + "/dec_keys?key_ID=" + keyID)
	if err != nil {
		return "", "", fmt.Errorf("failed to call ETSI API: %v", err)
	}

	key, id, err := parseResponse(resp)

	return key, id, err
}

func parseResponse(resp *http.Response) (string, string, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("ETSI API returned non-OK status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read ETSI API response: %v", err)
	}

	var response KeyContainer
	if err := json.Unmarshal(body, &response); err != nil {
		return "", "", fmt.Errorf("failed to parse ETSI API response: %v", err)
	}

	if len(response.Keys) != 1 {
		return "", "", fmt.Errorf("invalid length of keys array")
	}

	return response.Keys[0].Key, response.Keys[0].KeyID, nil
}
