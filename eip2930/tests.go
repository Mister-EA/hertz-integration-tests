package main

import (
	"context"
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/params"
	"hertzTests/config"
	"hertzTests/utils"
	"sync"

	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type TestCase struct {
	name               string       // name of the test
	validationFunction func() error // if error is nil validation is successful
}

var senderPrivateKey *ecdsa.PrivateKey
var senderAddress common.Address
var receiverPrivateKey *ecdsa.PrivateKey
var receiverAddress common.Address
var client *ethclient.Client
var err error

func init() {
	senderPrivateKey, err = crypto.HexToECDSA(config.SenderPrivateKeyHex)
	if err != nil {
		log.Fatal(err)
	}

	receiverPrivateKey, err = crypto.HexToECDSA(config.ReceiverPrivateKeyHex)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to an Ethereum client
	client, err = ethclient.Dial("http://localhost:8545")
	if err != nil {
		log.Fatal(err)
	}

	senderAddress = crypto.PubkeyToAddress(senderPrivateKey.PublicKey)
	receiverAddress = crypto.PubkeyToAddress(receiverPrivateKey.PublicKey)
}

func sendLegacyTransaction() (common.Hash, error) {
	nonce, err := client.PendingNonceAt(context.Background(), senderAddress)
	if err != nil {
		return common.Hash{}, err
	}
	// Set the amount of ETH to transfer
	value := big.NewInt(1000000000000000000) // 1 ETH

	// Set the gas price and limit
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return common.Hash{}, err
	}

	gasLimit := uint64(21000) // Standard gas limit for a transfer

	log.Println("Suggested gas price: ", gasPrice)

	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gasLimit,
		To:       &receiverAddress,
		Value:    value,
		Data:     []byte{},
	})
	// Sign the transaction with the sender's private key
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(config.ChainId), senderPrivateKey)
	if err != nil {
		return common.Hash{}, err
	}

	// Send the transaction to the Ethereum network
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return signedTx.Hash(), err

	}
	return signedTx.Hash(), nil
}

func sendAccessListTx() (common.Hash, error) {
	// Set the amount of ETH to transfer
	value := big.NewInt(1000000000000000000) // 1 ETH

	nonce, err := client.PendingNonceAt(context.Background(), senderAddress)
	if err != nil {
		return common.Hash{}, err
	}

	gasLimit := uint64(30000)

	tx := types.NewTx(&types.AccessListTx{
		ChainID: config.ChainId,
		Nonce:   nonce,
		To:      &receiverAddress,
		Value:   value,
		Gas:     gasLimit,

		Data: []byte{},
		AccessList: types.AccessList{{
			Address:     receiverAddress,
			StorageKeys: []common.Hash{{0}},
		}},
	})

	// Sign the transaction with the sender's private key
	signedTx, err := types.SignTx(tx, types.NewEIP2930Signer(config.ChainId), senderPrivateKey)
	if err != nil {
		return common.Hash{}, err
	}

	// Send the transaction to the Ethereum network
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return signedTx.Hash(), err
	}
	return signedTx.Hash(), nil
}

func testSendAccessListTx() error {
	txHash, err := sendAccessListTx()
	if err != nil {
		return err
	}
	receipt, err := utils.WaitForTransactionReceipt(client, txHash)
	if err != nil {
		return err
	}

	// Check receipt
	if receipt.Status != 1 {
		return fmt.Errorf("receipt.Status != 1. Receipt: %+v", receipt)
	}

	blockNr := receipt.BlockNumber
	block, err := client.BlockByNumber(context.Background(), blockNr)
	if err != nil {
		return err
	}

	expected := params.TxGas + params.TxAccessListAddressGas + params.TxAccessListStorageKeyGas

	if block.GasUsed() != expected {
		return fmt.Errorf("incorrect amount of gas spent: expected %d, got %d", expected, block.GasUsed())
	}
	return nil
}

// Runs the slice of test cases sequentially
func runTestCasesSequentially(testCases []TestCase) {
	for _, testCase := range testCases {
		err := testCase.validationFunction()
		if err != nil {
			log.Fatal(testCase.name, " FAILED: ", err)
		}
	}
}

func run2930Tests() {
	testCases := []TestCase{
		{
			name:               "testSendAccessListTx",
			validationFunction: testSendAccessListTx,
		},
	}
	runTestCasesSequentially(testCases)
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
	err = runPreHertz2930Tests()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("All Pre-Hertz tests passed!")
}

func runPreHertz2930Tests() error {
	_, err := sendAccessListTx()
	if err == nil {
		return fmt.Errorf("expected ErrTxTypeNotSupported but got `no error` instead")
	}
	if err != nil && err.Error() != types.ErrTxTypeNotSupported.Error() {
		return fmt.Errorf("expected ErrTxTypeNotSupported but got '%v' instead", err)
	}
	return nil
}

func postHertzTests() {
	log.Println("Post-Hertz tests:")
	log.Printf("Waiting for block number %v to start running the test cases...\n", config.PostHertzBlockNumber)
	err := utils.WaitForBlockNumber(client, config.PostHertzBlockNumber)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Block number %v reached, running test cases....\n", config.PostHertzBlockNumber)
	run2930Tests()
	fmt.Println("Tests passed!")
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
