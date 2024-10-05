FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y openssl libssl-dev make gcc curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

ENV GO_VERSION 1.22.3
RUN if ! command -v go &> /dev/null; then \
        echo "Go not found, installing Go $GO_VERSION..."; \
        curl -fsSL https://golang.org/dl/go$GO_VERSION.linux-amd64.tar.gz | tar -C /usr/local -xz && \
        ln -s /usr/local/go/bin/go /usr/bin/go && \
        ln -s /usr/local/go/bin/gofmt /usr/bin/gofmt; \
    else \
        echo "Go is already installed."; \
    fi

# Set Go environment variables
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:/usr/local/go/bin:$PATH

CMD ["bash"]