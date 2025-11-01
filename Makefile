KYBER_K ?= 4

CGO_CFLAGS := -I. -I./kyber-gake/ref -DKYBER_K=$(KYBER_K)
export CGO_CFLAGS

m:
	@echo "[Make]: Building member binary (KYBER_K=$(KYBER_K))..."
	@cd cluster_member && CGO_CFLAGS="$(CGO_CFLAGS)" go build -o ../member_pqgch

l:
	@echo "[Make]: Building leader binary (KYBER_K=$(KYBER_K))..."
	@cd leader && CGO_CFLAGS="$(CGO_CFLAGS)" go build -o ../leader_pqgch

clean:
	@echo "[Make]: Cleaning generated binaries..."
	@rm -f member_pqgch leader_pqgch

mock:
	@echo "[Make]: Running ETSI API mock server..."
	@cd mock_etsi && CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run *.go

config:
	@CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run util/cmd/main.go -m 3

gen_2ake:
	@echo "[Make]: Generating 2-AKE shared secret..."
	@CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run util/cmd/main.go -m 2

gen_kem:
	@echo "[Make]: Generating KEM keypairs..."
	@CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run util/cmd/main.go -c $(or $(n),1) -m 1

gen_ss:
	@echo "[Make]: Generating cluster shared secret..."
	@CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run util/cmd/main.go -m 0
