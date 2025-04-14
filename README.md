# Project Name

Brief one-line description of what your project does.

```
Hiten Mahalwar     | 10187026 | L01
Muhammad Shah      | 30122034 | L02
Andrew Howe        | 30113096 | L02
Karim Mansoor      | 30112539 | L02
Desmond O'Brien    | 30064340 | L01
```

## Table of Contents

- [Overview](#overview)
- [Dependencies](#dependencies)
- [Client Code](#client-code)
- [Server Code](#server-code)
- [Address Files](#address-files)

## Overview

MeChat's source code is split up into two directories:
1. Client
2. Server

## Dependencies
1. Golang
2. Node.js
3. A web browser

## Client Code

- Feature 1
- Feature 2
- Feature 3

## Server Code
### Client Golang Application
This is an application that is designed to run in the background silently
on a users machine. It handles communication between the client's machine
and the remote server. The source code is in:
```
client\back\client.go
```

To execute this code to connect to a remote server, use these commands:
```
cd client\back
go build
./main
```

### Client UI / React Application
This is UI hosted in a browser using React.js. The source code is quite involved, but it can all be found in:
```python
# Subcomponents, chatboxes, etc
client\front\src\components

# main pages for login, homepage, etc
client\front\src\pages
```

To execute this application, with Node.js installed, run the following commands
```python
cd client\front
npm install     # install node.js dependencies
npm start
```


## Server Code
The server is implemented strictly in Golang. The source code can be found in the following files:

```python
server\server.go   # main driver
server\FaultTolerance.go
server\SyncConsistency.go
server\RPCFuncs.go
```

## Address Files
Both the client and server use a text file that stored address of replicas:
```
client\back\replica_addrs.txt
server\replica_addrs.txt
```
**THESE FILES MUST BE IDENTICAL**

These are statitcally defined before the system is run. You may include as many addresses as you'd like. **The line number of an address corresponds to that replica's ID for the Bully algorithm**.

Ex:
```python
#replica_addrs.txt
------------------
10.0.0.1:12345  # ID = 0
10.0.0.2:12345  # ID = 1
```

If I was on the machine at address **10.0.0.2**, I would use the following commands to run a server replica:
```python
cd server
go build
./server 1      # Inlcude '1' as arg, my address is 1 based on file contents
```