package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"hertzTests/config"
	"hertzTests/utils"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Contract struct {
	ABI abi.ABI `json:"abi"`
	Bin string  `json:"bin"` // the bytecode
}

var CONTRACT_JSON_PATH = "./contracts/BaseFee.json"

var senderPrivateKey *ecdsa.PrivateKey
var senderAddress common.Address
var client *ethclient.Client
var baseFeeContract Contract
var err error

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

	// Read the contract ABI and byte code
	jsonFile, err := openFile(CONTRACT_JSON_PATH)
	if err != nil {
		log.Fatalf("Failed to open contract JSON: %v", err)
	}
	defer jsonFile.Close()

	// Parse the json file
	bytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.Fatalf("Failed to read bytes from JSON file: %v", err)
	}

	json.Unmarshal(bytes, &baseFeeContract)
	log.Printf("contract.ABI:\n %v\n\n", baseFeeContract.ABI)
	log.Printf("contract.Bin:\n  %v\n\n", baseFeeContract.Bin)
}

func openFile(filename string) (*os.File, error) {
	currentFilePath := getCurrentFilePath()
	dir := filepath.Dir(currentFilePath)
	combinedPath := filepath.Join(dir, filename)
	return os.Open(combinedPath)
}

// Get the filepath of the current go file running to use as the base
// of the relative path to the file we want to open.
// This way, the program will run correctly when called from any directory.
func getCurrentFilePath() string {
	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Cannot get the current file path")
	}
	return currentFilePath
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
		Gas:      300_000,
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

func deployBaseFeeContract() (common.Hash, common.Address, error) {
	txHash, contractAddress, err := deployContract(common.FromHex(baseFeeContract.Bin))
	if err != nil {
		return common.Hash{}, common.Address{}, err
	}

	receipt, err := utils.WaitForTransactionReceipt(client, txHash)
	if err != nil {
		return txHash, contractAddress, err
	}
	// Check that contract deployment has been successful
	if receipt.Status != 1 {
		err = fmt.Errorf("receipt.Status != 1. Receipt: %+v", receipt)
		return txHash, contractAddress, err
	}

	deployedCode, err := client.CodeAt(context.Background(), contractAddress, nil)
	if err != nil {
		return txHash, contractAddress, err
	}

	if len(deployedCode) <= 0 {
		err = fmt.Errorf("deployed code length is zero")
		return txHash, contractAddress, err
	}
	return txHash, contractAddress, nil
}

func testBaseFeeGlobalPreHertz(boundContract *bind.BoundContract) error {
	var result *big.Int
	err = boundContract.Call(nil, &[]interface{}{&result}, "basefee_global")
	expectedErrorMsg := "invalid opcode: BASEFEE"
	if err == nil {
		return fmt.Errorf("expected %s but got `no error` instead", expectedErrorMsg)
	}
	if err.Error() != expectedErrorMsg {
		return fmt.Errorf("Expected %s, got %s", expectedErrorMsg, err.Error())
	}
	return nil
}

func testBaseFeeAssemblyPreHertz(boundContract *bind.BoundContract) error {
	var result *big.Int
	err = boundContract.Call(nil, &[]interface{}{&result}, "basefee_inline_assembly")
	expectedErrorMsg := "invalid opcode: BASEFEE"
	if err == nil {
		return fmt.Errorf("Expected %s, got <nil>", expectedErrorMsg)
	}
	if err.Error() != expectedErrorMsg {
		return fmt.Errorf("Expected %s, got %s", expectedErrorMsg, err.Error())
	}
	return nil
}

func runPreHertzTests() error {
	txHash, contractAddress, err := deployBaseFeeContract()
	if err != nil {
		return err
	}
	log.Printf("BaseFee contract deployed at address = %v . txHash = %v\n", contractAddress, txHash)
	boundContract := bind.NewBoundContract(contractAddress, baseFeeContract.ABI, client, client, client)
	// Test basefee_global()
	err = testBaseFeeGlobalPreHertz(boundContract)
	if err != nil {
		return err
	}
	// Test basefee_inline_assembly()
	err = testBaseFeeAssemblyPreHertz(boundContract)
	if err != nil {
		return err
	}
	return nil
}

func testBaseFeeAssemblyPostHertz(boundContract *bind.BoundContract) error {
	var result *big.Int
	err = boundContract.Call(nil, &[]interface{}{&result}, "basefee_inline_assembly")
	if err != nil {
		return fmt.Errorf("Failed to call basefee_inline_assembly: %v", err)
	}
	log.Printf("basefee_inline_assembly() returned %s\n", result.String())
	return nil
}

func testBaseFeeGlobalPostHertz(boundContract *bind.BoundContract) error {
	var result *big.Int
	err = boundContract.Call(nil, &[]interface{}{&result}, "basefee_global")
	if err != nil {
		return err
	}

	log.Printf("basefee_global() returned  %s\n", result.String())
	if result.Cmp(common.Big0) != 0 {
		return fmt.Errorf("basefee_global() should have returned 0 but instead returned %s", result.String())
	}
	return nil
}

func runPostHertzTests() error {
	txHash, contractAddress, err := deployBaseFeeContract()
	if err != nil {
		return err
	}
	log.Printf("BaseFee contract deployed at address = %v . txHash = %v\n", contractAddress, txHash)
	boundContract := bind.NewBoundContract(contractAddress, baseFeeContract.ABI, client, client, client)
	// Test basefee_global()
	err = testBaseFeeGlobalPostHertz(boundContract)
	if err != nil {
		return err
	}
	// Tests basefee_inline_assembly()
	err = testBaseFeeAssemblyPostHertz(boundContract)
	if err != nil {
		return err
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
	err = runPreHertzTests()
	if err != nil {
		log.Fatal(err)
	}
}

func postHertzTests() {
	log.Println("Post-Hertz tests:")
	log.Printf("Waiting for block number %v to start running the test cases...\n", config.PostHertzBlockNumber)
	err := utils.WaitForBlockNumber(client, config.PostHertzBlockNumber)
	if err != nil {
		log.Fatal(err)
	}
	err = runPostHertzTests()
	if err != nil {
		log.Fatal(err)
	}
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
