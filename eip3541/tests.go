package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"sync"

	"hertzTests/config"
	"hertzTests/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var senderPrivateKey *ecdsa.PrivateKey
var senderAddress common.Address
var client *ethclient.Client
var err error

// The simplest bytecode that results in runtime bytecode of 0xef
var bytecodeDeploying0xEF = common.FromHex("0x60ef60005360016000f3")

func init() {
	senderPrivateKey, err = crypto.HexToECDSA(config.SenderPrivateKeyHex)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to an Ethereum client
	client, err = ethclient.Dial("http://localhost:8545")
	if err != nil {
		log.Fatal(err)
	}

	senderAddress = crypto.PubkeyToAddress(senderPrivateKey.PublicKey)
}

// Deploy contract with given bytecode. Returns (txHash, contractAddress, error)
func deployContract(bytecode []byte) (common.Hash, common.Address, error) {
	nonce, err := client.PendingNonceAt(context.Background(), senderAddress)
	if err != nil {
		return common.Hash{}, common.Address{}, err
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return common.Hash{}, common.Address{}, err
	}

	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      60_000,
		Value:    big.NewInt(0),
		Data:     bytecode,
	})
	signer := types.NewEIP155Signer(config.ChainId)
	signedTx, err := types.SignTx(tx, signer, senderPrivateKey)
	if err != nil {
		return common.Hash{}, common.Address{}, err
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return signedTx.Hash(), common.Address{}, err
	}
	contractAddress := crypto.CreateAddress(senderAddress, nonce)
	return signedTx.Hash(), contractAddress, nil
}

func test0xEFDDeploymentPreHertz() error {
	txHash, contractAddress, err := deployContract(bytecodeDeploying0xEF)
	if err != nil {
		return err
	}
	receipt, err := utils.WaitForTransactionReceipt(client, txHash)
	if err != nil {
		return err
	}
	// Check that contract deployment has been successful
	if receipt.Status != 1 {
		return fmt.Errorf("receipt.Status != 1. Receipt: %+v", receipt)
	}

	// Check that the contract code at the deployed contract address is 0xEF
	blockNr := receipt.BlockNumber
	contractCode, err := client.CodeAt(context.Background(), contractAddress, blockNr)
	if err != nil {
		return err
	}
	if !bytes.Equal(contractCode, common.FromHex("0xef")) {
		return fmt.Errorf("deployed contract code != 0xef. contract code = %v", common.Bytes2Hex(contractCode))
	}
	return nil
}

func test0xEFDeploymentPostHertz() error {
	txHash, _, err := deployContract(bytecodeDeploying0xEF)
	if err != nil {
		return err
	}
	receipt, err := utils.WaitForTransactionReceipt(client, txHash)
	if err != nil {
		return err
	}
	// Check that the transaction has failed
	if receipt.Status != 0 {
		return fmt.Errorf("receipt.Status != 0. Receipt: %+v", receipt)
	}
	return nil
}

func preHertzTests() {
	log.Println("Pre-Hertz tests:")
	blockNr, err := client.BlockNumber(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	if blockNr >= config.PostHertzBlockNumber {
		log.Fatalf("Too late to run pre-Hertz tests since current block number %v is after Hertz hard fork block %v.\n", blockNr, config.PostHertzBlockNumber)
	}
	log.Printf("Waiting for block number %v to start running the test cases...\n", config.PreHertzBlockNumber)
	err = utils.WaitForBlockNumber(client, config.PreHertzBlockNumber)
	if err != nil {
		log.Fatal(err)
	}
	// Run tests
	err = test0xEFDDeploymentPreHertz()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("All Pre-Hertz tests passed!")
}

func postHertzTests() {
	log.Println("Post-Hertz tests:")
	log.Printf("Waiting for block number %v to start running the test cases...\n", config.PostHertzBlockNumber)
	err := utils.WaitForBlockNumber(client, config.PostHertzBlockNumber)
	if err != nil {
		log.Fatal(err)
	}
	// Run tests
	err = test0xEFDeploymentPostHertz()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("All Post-Hertz tests passed!")
}

func main() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		preHertzTests()
	}()
	go func() {
		defer wg.Done()
		postHertzTests()
	}()
	wg.Wait()
	fmt.Println("ALL TESTS PASSED!")

}
