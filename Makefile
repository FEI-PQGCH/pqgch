CGO_CFLAGS_ALLOW="-fwrapv"
PREFERRED_CC=gcc-9
CC := $(shell command -v $(PREFERRED_CC) 2>/dev/null || echo gcc)

c%:
	@echo "[Make]: Running client$*..."
	@cd client && CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run *.go -config="../.config/c$*conf.json"

s%:
	@echo "[Make]: Running server$*..."
	@cd server && CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run *.go -config="../.config/s$*conf.json"

mock:
	@echo "[Make]: Running QKD mock server..."
	@cd qkd && CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run *.go

gen:
	@echo "[Make]: Generating the KEM keypairs..."
	@CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run util/cmd/main.go -c $(or $(n),1)