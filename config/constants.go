package config

import "math/big"

var PreHertzBlockNumber uint64 = 2   // a block number to run the pre-Hertz test cases
var PostHertzBlockNumber uint64 = 12 // a block number to run post-Hertz test cases
var ChainId = big.NewInt(1337)
var SenderPrivateKeyHex = "9b28f36fbd67381120752d6172ecdcf10e06ab2d9a1367aac00cdcd6ac7855d3"
var ReceiverPrivateKeyHex = "ddcd272732bfe889da92201da3527cb0faa4f3be06f5baa9e9269b700dfa2c2c"
