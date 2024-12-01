CGO_CFLAGS_ALLOW="-fwrapv"
PREFERRED_CC=gcc-9
BINARY_NAME=bin
CC := $(shell command -v $(PREFERRED_CC) 2>/dev/null || echo gcc)

build:
	@echo "make: Building project..."
	@cd client && CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go build -o ../$(BINARY_NAME)

clean:
	@echo "make: Cleaning up..."
	rm -f ${BINARY_NAME}

run: build
	@echo "make: Running the application..."
	@echo "make: Executing: $(BINARY_NAME)"
	@./$(BINARY_NAME)
	@echo "make: Removing the binary after execution..."
	@rm -f $(BINARY_NAME)

c%:
	@echo "make: Running client$*..."
	@cd client && CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run *.go -config="../.config/c$*conf.json"

s%:
	@echo "make: Running server$*..."
	@cd server && CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run *.go -config="../.config/s$*conf.json"

gen:
	@echo "make: Generating the KEM keypairs..."
	@CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run gakeutil/gake.go -p gen -c $(or $(n),1)

test: build
	@echo "make: Running GAKE test..."
	@CC=$(CC) CGO_CFLAGS_ALLOW=$(CGO_CFLAGS_ALLOW) go run gakeutil/gake.go -p test