CGO_CFLAGS_ALLOW="-fwrapv"
PREFERRED_CC=gcc-9
BINARY_NAME=bin
CC := $(shell command -v $(PREFERRED_CC) 2>/dev/null || echo gcc)

c%:
	@echo "[Make]: Running client$*..."
	@cd client && CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run *.go -config="../.config/c$*conf.json"

s%:
	@echo "[Make]: Running server$*..."
	@cd server && CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run *.go -config="../.config/s$*conf.json"

gen:
	@echo "[Make]: Generating the KEM keypairs..."
	@CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run gakeutil/gake.go -p gen -c $(or $(n),1)