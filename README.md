# Post-Quantum Group Chat using Authenticated Group Key Establishment

This repository contains the code for the Post-Quantum Group Chat.

## Directory Structure

- `client/` - code for the cluster members.
- `server/` - code for the servers, i.e. cluster leaders.
- `gake/` - Go wrapper code for the Kyber-GAKE protocol implementation in C.
- `gakeutil/` - helper program for key generation and GAKE testing.
- `shared/` - code shared between the client and server.
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
- `names` - the names of the clients in the cluster this server is the leader of.
- `servers` - the hostnames of other servers that are part of the communications.
- `index` - the index of this server.

For generating KEM keypairs, use the provided key generation program:

```
make gen n=3
```

This will generate three KEM keypairs in a JSON friendly format, ready to copy and paste into the configuration files.


To run the client, run the following command:
```
make cX
```

Where X is the number of the client you are starting.

To run the server, run the following command:
```
make sX
```

Where X is the number of the server you are starting.

When all is ready to go, you can start the protocol by typing `init` at each of the clients' terminals.

## TODO
- include the cluster leader in intra-cluster Kyber-GAKE
- finish the protocol as a whole, including all of the cluster leaders and their members
