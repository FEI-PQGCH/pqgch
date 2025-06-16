CC := $(shell command -v $(PREFERRED_CC) 2>/dev/null || echo gcc)
PREFERRED_CC := gcc-9

KYBER_K ?= 4

CGO_CFLAGS := -I. -I./kyber-gake/ref -DKYBER_K=$(KYBER_K)
export CGO_CFLAGS

c%:
	@echo "[Make]: Running client$* (KYBER_K=$(KYBER_K))…"
	@cd client && CC=$(CC) CGO_CFLAGS="$(CGO_CFLAGS)" go run *.go -config="../.config/c$*conf.json"

s%:
	@echo "[Make]: Running server$* (KYBER_K=$(KYBER_K))…"
	@cd server && CC=$(CC) CGO_CFLAGS="$(CGO_CFLAGS)" go run *.go -config="../.config/s$*conf.json"

mock:
	@echo "[Make]: Running QKD mock server..."
	@cd qkd && CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run *.go

gen:
	@echo "[Make]: Generating the KEM keypairs..."
	@CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run util/cmd/main.go -c $(or $(n),1)
