# BSC Hertz hard fork integration tests

This repository contains integration tests for the BEPs of the Hertz hard fork of BSC. The tests use a BSC Go client to send transactions to a running BSC node and check the outcome for a variety of different scenarios.



## Prerequisites
You need to be running a BSC node with Hertz, Berlin and London blocks activated at a certain block height.


The genesis config should define the block heights for `berlinBlock`, `londonBlock` and `hertzBlock` at the same height:
```
      "berlinBlock": 10,
      "londonBlock": 10,
      "hertzBlock": 10,
```

 Also the account that will send the transactions in the tests should be prefunded in the genesis file. This can be done by adding a line in the `alloc` section of the genesis file. For example:
```
   "9fB29AAc15b9A4B7F17c3385939b007540f4d791": {
      "balance": "0x84595161401484a000000"
    }
```

To test EIP-2930, the following memory address and contents are set in the `alloc` section of the genesis config to simplify the test.
```
      "0x000000000000000000000000000000000000aaaa" : {
        "balance": "0x0",
        "code": "0x58585454"
      }
```
This add the bytecode `PUSH`, `PUSH`,`SLOAD`,`SLOAD` at the address `0x000000000000000000000000000000000000aaaa`, which will be used for construction of accessListTx.

An example genesis file with all these requirements can be found in `genesis.json`.

### Start the BSC node
```
./build/bin/geth --datadir node_dir init genesis.json
./build/bin/geth --datadir node_dir console --http --http.corsdomain https://remix.ethereum.org --allow-insecure-unlock --http.api personal,eth,net,web3,debug --http.vhosts '*,localhost,host.docker.internal' --http.addr "0.0.0.0" --rpc.allow-unprotected-txs --networkid 1337 --miner.etherbase 0x9fb29aac15b9a4b7f17c3385939b007540f4d791 --vmdebug
```

### Start block production
```
personal.importRawKey("9b28f36fbd67381120752d6172ecdcf10e06ab2d9a1367aac00cdcd6ac7855d3", "123456") 
personal.unlockAccount("0x9fb29aac15b9a4b7f17c3385939b007540f4d791", "123456", 300000000) 
miner.start()
```


## Running the tests
You can tweak the parameters for the tests in the `config/constants.go` file. There you can configure at which block height to run the tests before and after the hard fork.


Double-check that the following lines are included in the `go.mod` file to ensure that the BSC Go client is used instead of the Ethereum Go client:

```
replace github.com/ethereum/go-ethereum v1.11.6 => github.com/bnb-chain/bsc v1.1.21

replace github.com/btcsuite/btcd => github.com/btcsuite/btcd v0.23.0

replace github.com/cometbft/cometbft => github.com/bnb-chain/greenfield-tendermint v0.0.0-20230417032003-4cda1f296fb2

replace github.com/grpc-ecosystem/grpc-gateway/v2 => github.com/prysmaticlabs/grpc-gateway/v2 v2.3.1-0.20210702154020-550e1cd83ec1

replace github.com/tendermint/tendermint => github.com/bnb-chain/tendermint v0.31.15
```


You can run the tests for a certain EIP using `go run <testFile.go>`. For example to run the EIP-1559 tests you should run `go run eip1559/tests.go`

**!!! Please make sure you run the tests before the hard fork block, otherwise the pre-Hertz test cases won't be able to run!**


