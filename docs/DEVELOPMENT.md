# Development Guide

## Working with Bitcoin Regtest Mode

```bash
# Fill out your username and password in env file at root directory
export BITCOIN_RPC_USER=
export BITCOIN_RPC_PASSWORD=

# Run Bitcoin Regtest Cluster (relative paths below assume cwd = docker/)
cd docker
docker compose --env-file ../.env -f bitcoin-regtest-cluster.yml up -d

# Setup CLI
alias btc1="docker exec -it bitcoin-node01 bitcoin-cli -regtest -rpcuser=$BITCOIN_RPC_USER -rpcpassword=$BITCOIN_RPC_PASSWORD -rpcwallet=sharedwallet"
alias btc2="docker exec -it bitcoin-node02 bitcoin-cli -regtest -rpcuser=$BITCOIN_RPC_USER -rpcpassword=$BITCOIN_RPC_PASSWORD -rpcwallet=sharedwallet"


##### SIMPLE END-TO-END: FORK + REORG + DOUBLE SPEND #####
# ===== CLEAN WALLET =====
btc1 -named unloadwallet wallet_name="sharedwallet" 2>/dev/null
btc2 -named unloadwallet wallet_name="sharedwallet" 2>/dev/null

btc1 loadwallet "sharedwallet" 2>/dev/null || btc1 createwallet "sharedwallet"
btc2 loadwallet "sharedwallet" 2>/dev/null || btc2 createwallet "sharedwallet"

# ===== MINE + SHARE KEY =====
addr=$(btc1 getnewaddress)
btc1 generatetoaddress 101 $addr

privkey=$(btc1 dumpprivkey $addr)
btc2 importprivkey $privkey
btc2 rescanblockchain

# ===== GET UTXO =====
utxo=$(btc1 listunspent | jq '.[0]')
txid=$(echo $utxo | jq -r '.txid')
vout=$(echo $utxo | jq -r '.vout')

# ===== CREATE ADDRESSES =====
addr1=$(btc1 getnewaddress)
addr2=$(btc2 getnewaddress)

# ===== PARTITION NETWORK =====
btc1 disconnectnode 172.21.0.11 2>/dev/null
btc2 disconnectnode 172.21.0.10 2>/dev/null

# ===== TX1 (node1) =====
raw1=$(btc1 createrawtransaction "[{\"txid\":\"$txid\",\"vout\":$vout}]" "{\"$addr1\":1}")
funded1=$(btc1 fundrawtransaction $raw1 | jq -r .hex)
signed1=$(btc1 signrawtransactionwithwallet $funded1 | jq -r .hex)
tx1=$(btc1 sendrawtransaction $signed1)

# ===== TX2 (node2 - double spend) =====
raw2=$(btc2 createrawtransaction "[{\"txid\":\"$txid\",\"vout\":$vout}]" "{\"$addr2\":1}")
funded2=$(btc2 fundrawtransaction $raw2 | jq -r .hex)
signed2=$(btc2 signrawtransactionwithwallet $funded2 | jq -r .hex)
tx2=$(btc2 sendrawtransaction $signed2)

# ===== MINE FORK =====
btc1 generatetoaddress 2 $(btc1 getnewaddress)
btc2 generatetoaddress 4 $(btc2 getnewaddress)

# ===== RECONNECT =====
btc1 addnode 172.21.0.11 onetry
btc2 addnode 172.21.0.10 onetry
sleep 3

# ===== RESULT =====
echo "==== RESULT ===="
echo "TX1:" $tx1
echo "TX2:" $tx2

echo "Node1 height:" $(btc1 getblockcount)
echo "Node2 height:" $(btc2 getblockcount)

echo "Check TX1:"
btc1 gettransaction $tx1 2>/dev/null || echo "TX1 ORPHANED"

echo "Check TX2:"
btc1 gettransaction $tx2 2>/dev/null || echo "TX2 NOT FOUND"

echo "Mempool:"
btc1 getrawmempool
```