FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

# Install necessary packages
RUN apt-get update && \
    apt-get install -y openssl libssl-dev make gcc curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Set Go version
ENV GO_VERSION=1.22.3

# Detect system architecture and set GO_ARCH accordingly
RUN ARCH=$(dpkg --print-architecture) && \
    if [ "$ARCH" = "amd64" ]; then \
        GO_ARCH="amd64"; \
    elif [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then \
        GO_ARCH="arm64"; \
    else \
        echo "Unsupported architecture: $ARCH"; exit 1; \
    fi && \
    echo "Installing Go $GO_VERSION for architecture: $GO_ARCH" && \
    curl -fsSL https://go.dev/dl/go$GO_VERSION.linux-$GO_ARCH.tar.gz | tar -C /usr/local -xz && \
    ln -s /usr/local/go/bin/go /usr/bin/go && \
    ln -s /usr/local/go/bin/gofmt /usr/bin/gofmt

# Set Go environment variables
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:/usr/local/go/bin:$PATH

CMD ["bash"]
