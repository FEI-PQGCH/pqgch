PREFERRED_CC := gcc-9
CC := $(shell command -v $(PREFERRED_CC) 2>/dev/null || echo gcc)

KYBER_K ?= 4

CGO_CFLAGS := -I. -I./kyber-gake/ref -DKYBER_K=$(KYBER_K)
export CGO_CFLAGS

c:
	@echo "[Make]: Building client binary (KYBER_K=$(KYBER_K))..."
	@cd client && CC=$(CC) CGO_CFLAGS="$(CGO_CFLAGS)" go build -o ../client_pqgch

l:
	@echo "[Make]: Building leader binary (KYBER_K=$(KYBER_K))..."
	@cd leader && CC=$(CC) CGO_CFLAGS="$(CGO_CFLAGS)" go build -o ../leader_pqgch

c%:
	@echo "[Make]: Running client$* (KYBER_K=$(KYBER_K))…"
	@cd client && CC=$(CC) CGO_CFLAGS="$(CGO_CFLAGS)" go run *.go -config="../.config/c$*conf.json"

l%:
	@echo "[Make]: Running leader$* (KYBER_K=$(KYBER_K))…"
	@cd leader && CC=$(CC) CGO_CFLAGS="$(CGO_CFLAGS)" go run *.go -config="../.config/s$*conf.json"

mock:
	@echo "[Make]: Running QKD mock leader..."
	@cd qkd && CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run *.go

gen_2ake:
	@echo "[Make]: Generating 2-AKE shared secret..."
	@CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run util/cmd/main.go -m 2

gen_kem:
	@echo "[Make]: Generating KEM keypairs..."
	@CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run util/cmd/main.go -c $(or $(n),1) -m 1

gen_ss:
	@echo "[Make]: Generating shared secret..."
	@CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run util/cmd/main.go -m 0
