# Post-Quantum Group Chat using Authenticated Group Key Establishment

This repository contains the code for the Post-Quantum Group Chat application.

## Directory Structure

- `client/` - code for the cluster members.
  - `client.go`
    - Handles user input, connects to the server, and facilitates message broadcasting and receiving.
    - Implements cryptographic protocol initialization (AKE).
  - `handler.go`
    - Processes received messages using handlers for different types (AKE, Xi, etc.).
    - Includes decryption for secure messages and displays plain text to the user.
- `server/` - code for the servers, i.e. cluster leaders.
  - `server.go`
    - Manages client connections, handles key exchange protocols, and routes messages within the cluster.
    - Establishes connections with neighboring servers for inter-cluster communication.
  - `routing.go`
    - Implements routing logic to send messages to specific clients, broadcast within the cluster, or forward to neighboring servers.
  - `handler.go`
    - Processes different message types (AKE, Broadcast, etc.) and forwards or broadcasts them as needed.
    - Facilitates cryptographic key exchange and secure message handling.
- `gake/` - Go wrapper code for the Kyber-GAKE protocol implementation in C.
- `gakeutil/` - helper program for key generation and GAKE testing.
- `shared/` - code shared between the client and server.
  - `config.go`
    - Parses user and server configuration files into structured objects.
    - Provides utility functions for accessing cryptographic keys, cluster information, and neighbor details.
  - `message.go`
    - Defines the Message structure and constants for message types.
    - Implements serialization and transmission of messages over network connections.
  - `protocol.go`
    - Manages cryptographic operations, including AKE key exchanges, shared secret generation, and message encryption/decryption.
    - Facilitates cluster-wide secure communication using shared secrets.
- `.samples/` - sample configuration files for both the clients and servers.
- `.config/` - directory for the configuration files for the clients and servers. IMPORTANT: you need to create this.

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

Then you can connect to the container with multiple shells using:

```bash
docker exec -it pqgch-instance bash
```

### Configuration

Before running the application, you need to set up the clients and servers. Create a `.config` directory and create some .json files. The files should be named cXconf.json and sXconf.json, where X is the number of the client or server you are configuring, starting from 1. The files should be in the same format as those in the `.samples` directory. Be careful to properly set the index in each of the config files. Client 1's index should be 0, client 2's 1 and so on. The same applies to servers.

#### Client config:
- `leadAddr` - the hostname of the cluster leader of the cluster this client belongs to.
- `names` - the names, or identifiers of ALL of the cluster members of the cluster this client belongs to.
- `index` - the index of the client in the cluster.
- `publicKeys` - the public keys of all of the cluster members of the cluster this client belongs to.
- `secretKey` - the secret key of this client.

#### Server config:
- `clusterConfig`
  - `names` - the names of the clients in the cluster this server is the leader of.
  - `index` - identifies this server in the cluster (e.g., 3 for "Server").
  - `publicKeys` - corresponds to the names list, providing each participant's public key for secure communication.
  - `secretKey` - the private key of the server for signing and decrypting data.
- `servers` - the hostnames of other servers that are part of the communications.
- `index` - identifies the server's position in the servers array.
- `publicKeys` - A separate list of public keys for servers, similar to the clusterConfig.publicKeys but specific to server-to-server operations.
- `secretKey` - The server's private key for server-to-server secure communications.

For generating KEM keypairs, use the provided key generation program:

```
make gen n=3
```

This will generate three KEM keypairs in a JSON friendly format, ready to copy and paste into the configuration files.

To run the server, run the following command:
```
make sX
```

Where X is the number of the server you are starting.

To run the client, run the following command:
```
make cX
```

Where X is the number of the client you are starting.

When everything is ready to go, you can start the protocol by typing `init` at each of the clients' terminals.

### Current Application State

* **Server Initialization**:
  * After starting servers and clients, each server attempts to connect to its right neighbor.
    * If the right neighbor is unavailable (e.g., the `make sX` command has not been executed yet), the server periodically retries the connection.
  * Each server knows the addresses of other servers through the `servers` field in its respective JSON configuration file.

* **Server-to-Server Connection**:
  * Servers exchange **AKE B messages** to establish secure connections among themselves.
    * The connection order is: left-to-right or right-to-left, eventually completing both.
  * Once connected, **Xi** messages are forwarded and received by all servers.
  * Each server collects all **Xs** from its peers.
  * Using these, the **MasterKey** is computed, and the **SkSid** is generated and printed.

* **Client Initialization**:
  * After clients are created and execute the `init` command, the protocol begins:
    * Clients send **AKE A messages** to the server and receive **AKE B messages** in response, establishing a secure connection.
  * When all clients are created (e.g., using `make c3` for three clients), the process completes:
    * The last client sends an **AKE A message** and receives an **AKE B message** from the server.
    * **Client 1 (c1)** sends **Xi** and receives **XiMsg** from the server, while other clients receive their respective **Xi messages**.

* **Message Exchange**:
  * At this stage, intra-cluster messaging among clients is partially functional:
    * Messages are successfully sent between clients.
    * However, receiving clients are currently unable to decrypt the messages.
      * **Reason**: Intra-cluster session keys are being used instead of the session keys generated by the main protocol.


## TODO
- clients should utilize session keys from the main protocol and be able to decrypt sent intra-cluster messages
- include the cluster leader in intra-cluster Kyber-GAKE
- finish the protocol as a whole, including all of the cluster leaders and their members
