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
	@go run client/client.go -config=".config/c$*conf.json"

s%:
	@echo "make: Running server$*..."
	@go run server/server.go -config=".config/s$*conf.json"