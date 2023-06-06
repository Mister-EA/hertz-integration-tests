#!/bin/bash

solc --combined-json abi,bin contracts/BaseFee.sol | jq '.contracts["contracts/BaseFee.sol:BaseFee"]' > contracts/BaseFee.json