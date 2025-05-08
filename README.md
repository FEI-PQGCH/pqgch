# Post-Quantum Group Chat using Authenticated Group Key Establishment

This repository contains the code for a post-quantum, authenticated group-chat application.
It uses Kyber-GAKE for secure, quantum-resistant group key establishment among clients and cluster leaders.


## Directory Structure

- `client/` - entrypoint for cluster member application.
  - `client.go`
    - Handles user input, connects to the server, and facilitates message broadcasting and receiving.
    - Implements cryptographic protocol initialization (AKE).
- `cluster_protocol/`  - multi-party GAKE logic within each cluster.
  - `protocol.go`
- `leader_protocol/`   - GAKE logic among cluster-leaders.
  - `protocol.go`
- `server/` - entrypoint for cluster‐leader application.
  - `server.go`
    - Manages client connections, handles key exchange protocols, and routes messages within the cluster.
    - Establishes connections with neighboring servers for inter-cluster communication.
  - `routing.go`
    - Implements routing logic to send messages to specific clients, broadcast within the cluster, or forward to neighboring servers.
- `gake/` - cgo wrapper for the Kyber-GAKE reference implementation.
  - `wrapper.go`
- `gakeutil/` - CLI for generating KEM keypairs (`make gen`).
  - `gake.go`
- `shared/` - common utilities for messaging, transport, config, and crypto.
  - `config.go`
    - Parses user and server configuration files into structured objects.
    - Provides utility functions for accessing cryptographic keys, cluster information, and neighbor details.
  - `message.go`
    - Defines the Message structure and constants for message types.
    - Implements serialization and transmission of messages over network connections.
  - `transport.go`
  - `crypto.go`
  - `etsi.go`
- `.samples/` - sample configuration files for both the clients and servers.
- `.config/` - your `cXconf.json` and `sXconf.json` (copy/modify from `.samples/`). IMPORTANT: you need to create this.
- `.keys/` - store your public‐keys and inter‐cluster key shares (copied from `.samples/`).

## Running the application

The prerequisites for building the application are:

```
openssl libssl-dev make gcc curl go
```

For easy setup, you can use VS Code's extension for Dev Containers, which will set up a Docker image containing 20.04 Ubuntu and all required dependencies for development and testing. The files required for this are located in the `.devcontainer` folder.

You can also run the Docker container manually. In the root directory, build the image:

```bash
docker build --tag pqgch:latest .devcontainer
```

Then start the container:

```bash
docker run -dit --name pqgch-instance -v $(pwd):/workspace pqgch:latest
```

If you want to forward the ports to your machine, use for example this:

```bash
docker run -dit \
  --name pqgch-instance \
  -p 9000:9000 \
  -p 9001:9001 \
  -p 9002:9002 \
  -v $(pwd):/workspace \
  pqgch:latest
```

Then you can connect to the container with multiple shells using:

```bash
docker exec -it pqgch-instance bash
```

### Configuration & Keys

Before running the application, you need to set up the clients and servers. Create `.config/` and `.keys/` directories. Copy `*.json` from `.samples/` → `.config/` and adjust: `c1conf.json, c2conf.json, …` for clients and `s1conf.json, s2conf.json, …` for servers. Copy the corresponding `.public_keys.json` and `inter*.txt` into `.keys/`.The files should be named cXconf.json and sXconf.json, where X is the number of the client or server you are configuring, starting from 1. The files should be in the same format as those in the `.samples` directory. Be careful to properly set the index in each of the config files. Client 1's index should be 0, client 2's 1 and so on. The same applies to servers.

#### Client config (`cXconf.json`):

- `leadAddr` - the hostname of the cluster leader of the cluster this client belongs to.
- `clusterConfig`
  - `names` - the names, or identifiers of ALL of the cluster members of the cluster this client belongs to.
  - `index` - the index of the client in the cluster.
  - `publicKeys` - the public keys of all of the cluster members of the cluster this client belongs to.
  - `secretKey` - Base64 Kyber secret key.

#### Server config (`sXconf.json`):

- `clusterConfig` (same as client)
  - `names` - the names of the clients in the cluster this server is the leader of.
  - `index` - identifies this server in the cluster (e.g., 3 for "Server").
  - `publicKeys` - corresponds to the names list, providing each participant's public key for secure communication.
  - `secretKey` - the private key of the server for signing and decrypting data.
- `keyLeft`   – optional QKD‐derived left neighbor secret.
- `keyRight`  – path or Base64 for right neighbor secret.
- `servers` - array of all leader addresses.
- `index` - this server’s position in the `servers` list.
- `secretKey` - Base64 Kyber secret key for this server.

##### Generating KEM keypairs
```
make gen n=3 # writes JSON‐encoded Base64 keypairs to stdout
```

This will generate three KEM keypairs in a JSON friendly format, ready to copy and paste into the configuration files.

##### Starting servers
```
make sX # start server X
```

Where X is the number of the server you are starting.

##### Starting clients
```
make cX # start client X
```

Where X is the number of the client you are starting.

When everything is ready to go, the protocol initializes automatically at each of the clients' terminals.

### Current Application State

- **Server Initialization**:

  - After starting servers and clients, each server attempts to connect to its right neighbor.
    - If the right neighbor is unavailable (e.g., the `make sX` command has not been executed yet), the server periodically retries the connection.
  - Each server knows the addresses of other servers through the `servers` field in its respective JSON configuration file.

- **Server-to-Server Connection**:

  - Servers exchange **AKE A** and **AKE B** messages to establish secure connections among themselves.
    - The connection order is left-to-right.
  - After exchanging the initial messages, **Xi** messages are forwarded and received by all servers.
  - Each server collects all **Xs** from its peers.
  - Using these, the **MasterKey** is computed, and the **SkSid** is generated and printed.

- **Client Initialization**:

  - After clients are created, `init` will automatically run and the protocol begins:
    - Clients send **AKE A messages** to the each other and receive **AKE B messages** in response, establishing a secure connection.
  - When all clients are exchange the initial messages, the process completes:
    - Clients broadcast their **Xs** and receive them from their peers.
    - After receiving all of the **Xs** from its peers, each client computes the **MasterKey** and the **SkSid** is generated and printed.

- **Message Exchange**:

  - Intra-cluster messaging among clients is fully functional:
    - Messages are successfully sent between clients and clusters.
    - Clients on receiving end from other clusters are currently able to encrypt and decrypt the messages.
      - Intra-cluster session keys are generated by the main protocol.

- **Message Queue**
  - Each server tracks per-client queues to handle offline/late messages.

### Run Time Dependencies

#### Dependencies used in this project:

- OpenSSL
- C standard library (libc)
- Go 1.20+
