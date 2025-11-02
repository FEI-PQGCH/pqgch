KYBER_K ?= 4

CGO_CFLAGS := -I. -I./kyber-gake/ref -DKYBER_K=$(KYBER_K)
export CGO_CFLAGS

m:
	@echo "building member binary (KYBER_K=$(KYBER_K))..."
	@cd cluster_member && CGO_CFLAGS="$(CGO_CFLAGS)" go build -o ../member_pqgch

l:
	@echo "building leader binary (KYBER_K=$(KYBER_K))..."
	@cd leader && CGO_CFLAGS="$(CGO_CFLAGS)" go build -o ../leader_pqgch

clean:
	@echo "cleaning generated binaries..."
	@rm -f member_pqgch leader_pqgch

mock:
	@echo "running ETSI API mock server..."
	@cd mock_etsi && go run *.go

config:
	@go run util/cmd/main.go -m 3

gen_2ake:
	@echo "generating 2-AKE shared secret..."
	@go run util/cmd/main.go -m 2

gen_kem:
	@echo "generating KEM keypairs..."
	@go run util/cmd/main.go -c $(or $(n),1) -m 1

gen_ss:
	@echo "generating cluster shared secret..."
	@go run util/cmd/main.go -m 0
