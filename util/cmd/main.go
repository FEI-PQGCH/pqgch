package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"pqgch/gake"
	"pqgch/util"
	"strconv"
	"strings"
)

func main() {
	count := flag.Int("c", 1, "number of keypairs to generate")
	mode := flag.Int("m", 0, "mode for generation - KEM keypair (0), QKD shared secret (1), 2-AKE shared secret (2), whole configuration (3)")
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
		keyPairs := genKemKeypairs(*count)

		if *count == 1 {
			fmt.Println("printing public key")
			fmt.Printf("{\n\"key\": \"%s\"\n}\n", keyPairs[0].pk)

			fmt.Println("printing secret key")
			fmt.Printf("{\n\"key\": \"%s\"\n}\n", keyPairs[0].sk)
		} else {
			fmt.Printf("\nprinting public keys 0..%d\n\n", *count-1)
			fmt.Println("{\"publicKeys\": [")
			for i := range *count {
				fmt.Printf("\"%s\"", keyPairs[i].pk)
				if i < *count-1 {
					fmt.Println(",")
				}
			}
			fmt.Println("\n]\n}")

			fmt.Printf("\nprinting secret keys 0..%d\n", *count-1)
			for i := range *count {
				fmt.Printf("{\n\"key\": \"%s\"\n}\n", keyPairs[i].sk)
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
	// Generate whole configuration.
	case 3:
		generateConfig()
	}

}

type KeyPair struct {
	pk string
	sk string
}

func genKemKeypairs(n int) []KeyPair {
	keyPairs := make([]KeyPair, n)
	for i := range n {
		keyPair := gake.GetKemKeyPair()

		var keyPairString KeyPair
		keyPairString.pk = base64.StdEncoding.EncodeToString(keyPair.Pk[:])
		keyPairString.sk = base64.StdEncoding.EncodeToString(keyPair.Sk[:])
		keyPairs[i] = keyPairString
	}
	return keyPairs
}

var (
	configPath      = "config.json"
	leftCryptoPath  = "left_pk.json"
	rightCryptoPath = "right_pk.json"
	skPath          = "sk.json"
	pksPath         = "pks.json"
	clusterPksPath  = "cluster_pks.json"
	clusterSkPath   = "cluster_sk.json"
)

func generateConfig() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("enter path to create configuration at: ")
	prefix, _ := reader.ReadString('\n')
	prefix = strings.TrimSpace(prefix)

	fmt.Print("enter router server address (e.g., localhost:9000): ")
	server, _ := reader.ReadString('\n')
	server = strings.TrimSpace(server)

	fmt.Print("how many clusters? ")
	nClustersStr, _ := reader.ReadString('\n')
	nClusters, _ := strconv.Atoi(strings.TrimSpace(nClustersStr))
	if nClusters <= 1 {
		fmt.Println("you need atleast 2 clusters")
		return
	}

	leaderKeypairs := genKemKeypairs(nClusters)

	for i := range nClusters {
		fmt.Printf("\ncluster %d:\n", i+1)
		fmt.Print("number of members in this cluster? ")
		nMembersStr, _ := reader.ReadString('\n')
		nMembers, err := strconv.Atoi(strings.TrimSpace(nMembersStr))
		if nMembers <= 0 || err != nil {
			fmt.Println("defaulting to 1 member cluster (only leader)")
			nMembers = 1
		}

		leaderName := fmt.Sprintf("leader%d", i+1)

		skFilePath := filepath.Join(prefix, leaderName, skPath)
		leftPkFilePath := filepath.Join(prefix, leaderName, leftCryptoPath)
		rightPkFilePath := filepath.Join(prefix, leaderName, rightCryptoPath)

		leftIndex := (i - 1 + nClusters) % nClusters
		rightIndex := (i + 1 + nClusters) % nClusters

		writeKey(skFilePath, leaderKeypairs[i].sk)
		writeKey(leftPkFilePath, leaderKeypairs[leftIndex].pk)
		writeKey(rightPkFilePath, leaderKeypairs[rightIndex].pk)

		clusterKeyPairs := genKemKeypairs(nMembers)
		var clusterPks []string
		for _, keyPair := range clusterKeyPairs {
			clusterPks = append(clusterPks, keyPair.pk)
		}
		clusterPksFilePath := filepath.Join(prefix, leaderName, clusterPksPath)
		writeJSON(clusterPksFilePath, map[string][]string{
			"publicKeys": clusterPks,
		})
		clusterSkFilePath := filepath.Join(prefix, leaderName, clusterSkPath)
		writeKey(clusterSkFilePath, clusterKeyPairs[nMembers-1].sk)

		leaderConfig := util.BaseConfig{
			Server:    server,
			Name:      leaderName,
			ClusterID: i,
			Leader: &util.LeaderConfig{
				NClusters:   nClusters,
				LeftCrypto:  leftCryptoPath,
				RightCrypto: rightCryptoPath,
				SecretKey:   skPath,
			},
		}

		if nMembers > 1 {
			leaderConfig.Cluster = &util.ClusterConfig{
				NMembers:   nMembers,
				MemberID:   nMembers - 1,
				PublicKeys: clusterPksPath,
				SecretKey:  clusterSkPath,
			}
		}
		leaderConfigFilePath := filepath.Join(prefix, leaderName, configPath)
		writeJSON(leaderConfigFilePath, leaderConfig)

		for j := range nMembers - 1 {
			memberName := fmt.Sprintf("member%d_cluster%d", j+1, i+1)
			clusterPksFilePath := filepath.Join(prefix, memberName, pksPath)
			writeJSON(clusterPksFilePath, map[string][]string{
				"publicKeys": clusterPks,
			})
			clusterSkFilePath := filepath.Join(prefix, memberName, skPath)
			writeKey(clusterSkFilePath, clusterKeyPairs[j].sk)

			memberConfig := util.BaseConfig{
				Server:    server,
				Name:      memberName,
				ClusterID: i,
				Cluster: &util.ClusterConfig{
					MemberID:   j,
					NMembers:   nMembers,
					PublicKeys: pksPath,
					SecretKey:  skPath,
				},
			}

			memberConfigPath := filepath.Join(prefix, memberName, configPath)
			writeJSON(memberConfigPath, memberConfig)
		}
	}

	fmt.Println("\nall configs generated successfully to: " + prefix)
}

func writeKey(path, content string) {
	os.MkdirAll(filepath.Dir(path), os.ModePerm)
	data, _ := json.MarshalIndent(map[string]string{
		"key": content,
	}, "", "  ")
	os.WriteFile(path, data, 0666)
}

func writeJSON(path string, v any) {
	os.MkdirAll(filepath.Dir(path), os.ModePerm)
	data, _ := json.MarshalIndent(v, "", "  ")
	os.WriteFile(path, data, 0666)
}
