# Go-Vocchain

Official Golang implementation of the Vocchain protocol.

 Binary archives are published at https://www.voconline.io/#download

### Runing
```
voc-core --conf=~/.voc/config.conf --datadir=~/.voc
```
PROGRAMMATICALLY INTERFACING VOC-CORE NODES 
User flag
```
--listen Enable the HTTP-RPC server  
--host HTTP-RPC server listening interface (default: localhost)  
--port HTTP-RPC server listening port (default: 9333)  
--user  HTTP-RPC server UserName  
--password HTTP-RPC server Password  
```
User config 
```
[rpc]
host=127.0.0.1  
port=9333  
rpcuser=345345354  
rpcpassword=12323234  
rpcallowip=127.0.0.1,192.168.1.222,0.0.0.0
Note: Please understand the security implications of opening up an HTTP/WS based transport before doing so! Hackers on the internet are actively trying to subvert VocChain nodes with exposed APIs! Further, all browser tabs can access locally running web servers, so malicious web pages could try to subvert locally available APIs!

CONFIG EXAMPLE
[data]
datadir=d:/Voctest
listen=1
server=1

[p2p]
# p2p-listen-endpoint = 0.0.0.0:9332
# p2p-server-address =
# p2p-peer-address =
# p2p-max-nodes-per-host = 1
# agent-name = "VOC Node
# allowed-connection = any
# peer-key =
# sync-fetch-span = 100 
# max-clients = 25
# connection-cleanup-period = 30
# max-cleanup-time-msec = 10
# network-version-match = 0

[rpc]
host=127.0.0.1
port=9333
rpcuser=345345354
rpcpassword=12323234
rpcallowip=127.0.0.1,192.168.1.2

[wallet]
fee=0.001
```