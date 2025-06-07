package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"pqgch/shared"
	"strconv"
	"strings"
)

type Key struct {
	KeyID string `json:"key_ID"`
	Key   string `json:"key"`
}

type KeyContainer struct {
	Keys []Key `json:"keys"`
}

// Map of Key_ID -> Key
var keyStore = map[string]string{}

func main() {
	http.HandleFunc("/etsi/", keysHandler)
	log.Println("Mock ETSI QKD API running on :8080")
	http.ListenAndServe(":8080", nil)
}

func keysHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL)

	path := strings.TrimPrefix(r.URL.Path, "/etsi/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "invalid URL", http.StatusBadRequest)
		return
	}

	saeID := parts[0] // TODO: ignored for now.
	action := parts[1]

	// We only allow GET requests for simplicity.
	// This should be enough for our use case anyway.
	if r.Method == http.MethodGet {
		switch action {
		case "enc_keys":
			handleEncKeys(w, r, saeID)
		case "dec_keys":
			handleDecKeys(w, r, saeID)
		default:
			http.Error(w, "unknown endpoint", http.StatusNotFound)
		}
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleEncKeys(w http.ResponseWriter, r *http.Request, _ string) {
	num, err := strconv.Atoi(r.URL.Query().Get("number"))
	if err != nil {
		http.Error(w, "malformed input", http.StatusBadRequest)
		return
	}

	size, err := strconv.Atoi(r.URL.Query().Get("size"))
	if err != nil {
		http.Error(w, "malformed input", http.StatusBadRequest)
		return
	}

	// Generate the key/keys and send them to the client.
	keys := getKeys(num, size)
	respond(w, keys)
}

func handleDecKeys(w http.ResponseWriter, r *http.Request, _ string) {
	keyID := r.URL.Query().Get("key_ID")
	// Look the key up in the key store.
	if key, found := keyStore[keyID]; found {
		respond(w, []Key{{KeyID: keyID, Key: key}})
		return
	}
	http.Error(w, "key corresponding to this key_ID not found", http.StatusBadRequest)
}

func getKeys(n, size int) []Key {
	var keys []Key
	// Generate n keys (should be just 1).
	for range n {
		id := shared.UniqueID()
		key := make([]byte, size/8)
		rand.Read(key)
		encoded := base64.StdEncoding.EncodeToString(key)
		keyStore[id] = encoded
		keys = append(keys, Key{
			KeyID: id,
			Key:   encoded,
		})
	}

	return keys
}

func respond(w http.ResponseWriter, keys []Key) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(KeyContainer{Keys: keys})
}
