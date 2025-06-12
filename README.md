# Post-Quantum Group Chat using Authenticated Group Key Establishment

This repository contains the code for a post-quantum, authenticated group chat application.

It uses Kyber-GAKE for secure, quantum-resistant group key establishment among clients and cluster leaders. It also offers the possibility of executing parts of the protocol through a QKD system.

## Table of Contents

1. [Running the application](#running-the-application)

   1. [Configuration and Keys](#configuration-and-keys)
   2. [Running locally (Linux)](#running-locally-linux)
   3. [Running using Docker](#running-using-docker)
   4. [Running using Dev Containers in VS Code](#running-using-dev-containers-in-vs-code)

2. [Configuration Files](#configuration-files)
   1. [Client](#client-configuration-cxconfjson)
   2. [Server](#server-configuration-sxconfjson)
3. [Mock ETSI QKD API server](#mock-etsi-qkd-api-server)

## Running the application

To run the application, you first need to prepare the configuration files.

### Configuration and Keys

Before running the application, you need to set up the configuration files for clients and servers.

First, create a `.config/` directory. Now, you can either create your own configuration files and customize them to your needs, or you can use the configuration files from the `.samples/` directory.

> **_NOTE:_** The `.samples/` directory contains configuration files for a setup of 3 cluster leaders (so 3 clusters), with the first cluster having 3 cluster members (4 in total including the cluster leader) and the second and third cluster having 1 cluster member each (2 if we include the cluster leaders). Cluster leaders 0 and 1 communicate via a mock ETSI QKD server, while the interaction between cluster leaders 1 - 2 and 2 - 0 is via post-quantum 2-AKE.

Then, you can copy the `*.json` files from the `.samples/` directory to your freshly created `.config/` directory.

> **_NOTE:_** The configuration files in the `.samples/` directory refer to keys in the `.keys/` directory. You can use different keys. You can generate Kyber KEM keypairs using the helper utility `make gen`. So for example if you want to generate 3 keypairs, you can run `make gen n=3`, which will print out the keys in a config-friendly way. For the symmetric keys (here found in the `.keys/interX_key.txt` files), you just need to enter some random hexadecimal string of 64 characters (for a 32 byte key).

> **_IMPORTANT:_** This configuration is for a **local** setup on a single computer. If you want to use the application on mutliple computers on a network, you need to change the configuration files accordingly (mainly the IP adresses). You can also customize the number of clients and cluster leaders (servers).

Afterwards you have multiple options for running the application.

### Running locally (Linux)

The prerequisites for building the application are:

```
openssl libssl-dev make gcc curl go
```

If you are using the default configuration from the `.samples/` directory, you first need to start the mock ETSI QKD server:

```
make mock
```

Then you can run the server by running:

```
make sX
```

Where X is the number of the server (for example `make s1`).

Similarly for clients:

```
make cX
```

Where X is the number of the client (for example `make c1`).

### Running using Docker

Instead of locally installing the dependencies, you can use Docker. In the root directory of the project, run:

```bash
docker build --tag pqgch:latest .devcontainer
```

Then start the container:

```bash
docker run -dit --name pqgch-instance -v $(pwd):/workspace pqgch:latest
```

Or if you want to forward the ports to your machine (for usage on a network, so others can reach the container from outside), use for example this:

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

Then you can proceed by running the application using the `make` commands as explained in [Running locally (Linux)](#running-locally-linux)

### Running using Dev Containers in VS Code

For easy setup, you can use VS Code's extension for Dev Containers, which will set up a Docker image containing 20.04 Ubuntu and all required dependencies for development and testing. The files required for this are located in the `.devcontainer` folder.

Use for example the following tutorial for this: [Dev Containers Tutorial](https://code.visualstudio.com/docs/devcontainers/tutorial).

Afterwards, connect to the container using multiple shells and proceed by running the `make` commands as explained in [Running locally (Linux)](#running-locally-linux)

## Configuration files

This section explains the format and contents of the configuration files. These are plain JSON files.

### Client configuration (`cXconf.json`):

Here by _cluster_ we mean the cluster **this** client belongs to.

- `leadAddr` - the IP address of the cluster leader of the cluster
- `clusterConfig`
  - `names` - the names, or identifiers of ALL of the cluster members of the cluster
  - `index` - the index of the client in the cluster
  - `publicKeys` - the path to the file containing the public keys of all of the cluster members of the cluster
  - `secretKey` - base64 encoded Kyber KEM secret key

### Server configuration (`sXconf.json`):

- `clusterConfig` (same as client, _cluster_ also means the same thing)
  - `names` - the names, or identifiers of ALL of the cluster members of the cluster.
  - `index` - the index of the cluster leader in the cluster
  - `publicKeys` - the path to the file containing the public keys of all of the cluster members of the cluster
  - `secretKey` - base64 encoded Kyber KEM secret key
- `keyLeft` – left neighbor crypto info (see NOTE)
- `keyRight` – right neighbor crypto info (see NOTE)
- `servers` - a list of all cluster leader addresses
- `index` - this server’s position in the `servers` list.
- `secretKey` - base64 encoded Kyber KEM secret key

> **_NOTE:_** The `keyLeft` and `keyRight` properties can be one of the following:
>
> - a base64 encoded Kyber KEM public key of the corresponding neighbor (as generated by `make gen`)
> - an URL of an ETSI QKD API server. In this case the string has to begin with `url`, followed by a single space and then the URL itself (for example `url http://localhost:8080/etsi/`)
> - a path to a file containing a 32 byte secret key as a hexadecimal string of 64 characters. In this case the string has to begin with `path`, followed by a single space and then the path of the file on the local file system (for example `path ../.keys/key1.txt`)

> **_IMPORTANT:_** It is important for the cluster leaders' `keyLeft` and `keyRight` properties to match up. So as an example using the Kyber KEM public keys, if cluster leader 1 has as its right neighbor cluster leader 2, the cluster leader's 1 `keyRight` property contains the public key of cluster leader 2, and cluster leader's 2 `keyLeft` property contains the public key of cluster leader 1.

## Mock ETSI QKD API server

The project contains a mock ETSI QKD API server. It is a simple HTTP server providing the `Get Keys` and `Get Keys with IDs` endpoints from the [ETSI standard documentation](https://www.etsi.org/deliver/etsi_gs/QKD/001_099/014/01.01.01_60/gs_QKD014v010101p.pdf).

You can use the following CURL commands to interact with the mock QKD server:

```
curl -X GET "http://localhost:8080/etsi/DUMMY_ID/enc_keys?number=1&size=256"
```

```
curl -X GET "http://localhost:8080/etsi/DUMMY_ID/dec_keys?key_ID=d21fe47e2ecb684b95720d740de3b1d9"
```
