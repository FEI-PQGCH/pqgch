package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"pqgch/gake"
)

func main() {
	count := flag.Int("c", 1, "number of keypairs to generate")
	mode := flag.Int("m", 0, "mode for generation - KEM keypair (0), QKD shared secret (1), 2-AKE shared secret (2)")
	flag.Parse()

	switch *mode {
	// Generate shared secret
	case 0:
		key := make([]byte, 2*gake.SsLen)
		_, err := rand.Read(key)
		if err != nil {
			panic(err)
		}

		encodedKey := base64.StdEncoding.EncodeToString(key)
		fmt.Printf("{\n\"key\": \"%s\"\n}\n", encodedKey)
	// Generate KEM keypairs.
	case 1:
		keyPairs := make([]gake.KemKeyPair, *count)
		for i := range *count {
			keyPairs[i] = gake.GetKemKeyPair()
		}

		if *count == 1 {
			fmt.Println("printing public key")
			fmt.Printf("{\n\"key\": \"%s\"\n}\n", base64.StdEncoding.EncodeToString(keyPairs[0].Pk[:]))

			fmt.Println("printing secret key")
			fmt.Printf("{\n\"key\": \"%s\"\n}\n", base64.StdEncoding.EncodeToString(keyPairs[0].Sk[:]))
		} else {
			fmt.Printf("\nprinting public keys 0..%d\n\n", *count-1)
			fmt.Println("{\"publicKeys\": [")
			for i := range *count {
				fmt.Printf("\"%s\"", base64.StdEncoding.EncodeToString(keyPairs[i].Pk[:]))
				if i < *count-1 {
					fmt.Println(",")
				}
			}
			fmt.Println("\n]\n}")

			fmt.Printf("\nprinting secret keys 0..%d\n", *count-1)
			for i := range *count {
				fmt.Printf("{\n\"key\": \"%s\"\n}\n", base64.StdEncoding.EncodeToString(keyPairs[i].Sk[:]))
			}
		}
	// Generate 2-AKE shared secret.
	case 2:
		key := make([]byte, gake.SsLen)
		_, err := rand.Read(key)
		if err != nil {
			panic(err)
		}

		encodedKey := base64.StdEncoding.EncodeToString(key)
		fmt.Printf("{\n\"key\": \"%s\"\n}\n", encodedKey)
	}
}
