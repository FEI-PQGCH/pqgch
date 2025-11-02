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

func main() {
	count := flag.Int("c", 1, "number of keypairs to generate")
	mode := flag.Int("m", 0, "mode for generation - KEM keypair (0), QKD shared secret (1), 2-AKE shared secret (2), whole configuration (3)")
	flag.Parse()

	switch *mode {
	case 0:
		generateKey(2 * gake.SsLen)
	case 1:
		generateKeyPairs(*count)
	case 2:
		generateKey(gake.SsLen)
	case 3:
		generateConfig()
	}

}

func generateKey(n int) {
	key := make([]byte, n)
	_, _ = rand.Read(key)
	encodedKey := base64.StdEncoding.EncodeToString(key)
	writeJSON(map[string]string{
		"key": encodedKey,
	})
}

func generateKeyPairs(n int) {
	keyPairs := genKemKeypairs(n)

	if n == 1 {
		fmt.Println("printing public key")
		writeJSON(map[string]string{
			"key": keyPairs[0].pk,
		})

		fmt.Println("printing secret key")
		writeJSON(map[string]string{
			"key": keyPairs[0].sk,
		})
		return
	}

	fmt.Printf("\nprinting public keys 0..%d\n\n", n-1)
	var clusterPks []string
	for i := range n {
		clusterPks = append(clusterPks, keyPairs[i].pk)
	}
	writeJSON(map[string][]string{
		"publicKeys": clusterPks,
	})

	fmt.Printf("\nprinting secret keys 0..%d\n\n", n-1)
	for i := range n {
		writeJSON(map[string]string{
			"key": keyPairs[i].sk,
		})
	}
}

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
		fmt.Print("number of members in this cluster (including leader)? ")
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

		writeJSONToFile(skFilePath, map[string]string{
			"key": leaderKeypairs[i].sk,
		})
		writeJSONToFile(leftPkFilePath, map[string]string{
			"key": leaderKeypairs[leftIndex].pk,
		})
		writeJSONToFile(rightPkFilePath, map[string]string{
			"key": leaderKeypairs[rightIndex].pk,
		})

		clusterKeyPairs := genKemKeypairs(nMembers)
		var clusterPks []string
		for _, keyPair := range clusterKeyPairs {
			clusterPks = append(clusterPks, keyPair.pk)
		}

		leaderConfig := util.BaseConfig{
			Server:    server,
			Name:      leaderName,
			ClusterID: &i,
			Leader: &util.LeaderConfig{
				NClusters:   &nClusters,
				LeftCrypto:  leftCryptoPath,
				RightCrypto: rightCryptoPath,
				SecretKey:   skPath,
			},
		}

		if nMembers > 1 {
			clusterPksFilePath := filepath.Join(prefix, leaderName, clusterPksPath)
			writeJSONToFile(clusterPksFilePath, map[string][]string{
				"publicKeys": clusterPks,
			})
			clusterSkFilePath := filepath.Join(prefix, leaderName, clusterSkPath)
			writeJSONToFile(clusterSkFilePath, map[string]string{
				"key": clusterKeyPairs[nMembers-1].sk,
			})

			memberID := nMembers - 1
			leaderConfig.Cluster = &util.ClusterConfig{
				NMembers:   &nMembers,
				MemberID:   &memberID,
				PublicKeys: clusterPksPath,
				SecretKey:  clusterSkPath,
			}
		}
		leaderConfigFilePath := filepath.Join(prefix, leaderName, configPath)
		writeJSONToFile(leaderConfigFilePath, leaderConfig)

		for j := range nMembers - 1 {
			memberName := fmt.Sprintf("member%d_cluster%d", j+1, i+1)
			clusterPksFilePath := filepath.Join(prefix, memberName, pksPath)
			writeJSONToFile(clusterPksFilePath, map[string][]string{
				"publicKeys": clusterPks,
			})
			clusterSkFilePath := filepath.Join(prefix, memberName, skPath)
			writeJSONToFile(clusterSkFilePath, map[string]string{
				"key": clusterKeyPairs[j].sk,
			})

			memberConfig := util.BaseConfig{
				Server:    server,
				Name:      memberName,
				ClusterID: &i,
				Cluster: &util.ClusterConfig{
					MemberID:   &j,
					NMembers:   &nMembers,
					PublicKeys: pksPath,
					SecretKey:  skPath,
				},
			}

			memberConfigPath := filepath.Join(prefix, memberName, configPath)
			writeJSONToFile(memberConfigPath, memberConfig)
		}
	}

	fmt.Println("\nall configs generated successfully to: " + prefix)
}

func writeJSONToFile(path string, v any) {
	os.MkdirAll(filepath.Dir(path), os.ModePerm)
	data, _ := json.MarshalIndent(v, "", "  ")
	os.WriteFile(path, data, 0666)
}

func writeJSON(v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}
