package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"pqgch/gake"
	"strconv"
	"time"
)

type Key struct {
	KeyID string `json:"key_ID"`
	Key   string `json:"key"`
}

type KeyContainer struct {
	Keys []Key `json:"keys"`
}

func getKey(endpoint string) (string, string, error) {
	resp, err := http.Get(endpoint + "/enc_keys?number=1&size=" + strconv.Itoa(gake.SsLen*8*2))
	if err != nil {
		return "", "", fmt.Errorf("failed to call ETSI API: %w", err)
	}

	key, id, err := parseResponse(resp)

	return key, id, err
}

// Make a request for the key with keyID to the ETSI QKD API.
func getKeyByID(endpoint, keyID string) (string, string, error) {
	resp, err := http.Get(endpoint + "/dec_keys?key_ID=" + keyID)
	if err != nil {
		return "", "", fmt.Errorf("failed to call ETSI API: %w", err)
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

func RequestKey(url string, isLeader bool) (Message, Message) {
	var key, keyID string
	for {
		var err error
		key, keyID, err = getKey(url)
		if err != nil {
			LogError(fmt.Sprintf("Requesting QKD key: %v. Retrying...", err))
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}

	// Process the received key.
	var msgType int
	if isLeader {
		msgType = QKDRightKeyMsg
	} else {
		msgType = QKDClusterKeyMsg
	}

	keyMsg := Message{
		Type:    msgType,
		Content: key,
	}

	IDMsg := Message{
		Type:    QKDIDMsg,
		Content: keyID,
	}

	return keyMsg, IDMsg
}

func RequestKeyByID(url, id string, isLeader bool) Message {
	var key string
	for {
		var err error
		key, _, err = getKeyByID(url, id)
		if err != nil {
			LogError(fmt.Sprintf("Requesting QKD key by ID: %v. Retrying...", err))
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}

	// Process the received key.
	var msgType int
	if isLeader {
		msgType = QKDLeftKeyMsg
	} else {
		msgType = QKDClusterKeyMsg
	}

	msg := Message{
		Type:    msgType,
		Content: key,
	}
	return msg
}
