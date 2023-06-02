package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"hertzTests/config"
	"hertzTests/utils"
	"sync"

	// "crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	// "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
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

func sendDynamicFeeTx(gasFeeCap, gasTipCap *big.Int) (common.Hash, error) {
	// Set the amount of ETH to transfer
	value := big.NewInt(1000000000000000000) // 1 ETH

	nonce, err := client.PendingNonceAt(context.Background(), senderAddress)
	if err != nil {
		return common.Hash{}, err
	}

	log.Println("GasFeeCap: ", gasFeeCap)
	log.Println("GasTipCap: ", gasTipCap)
	gasLimit := uint64(21000) // Standard gas limit for a transfer

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:    config.ChainId,
		Nonce:      nonce,
		To:         &receiverAddress,
		Value:      value,
		Gas:        gasLimit,
		GasFeeCap:  gasFeeCap,
		GasTipCap:  gasTipCap,
		Data:       []byte{},
		AccessList: nil,
	})

	// Sign the transaction with the sender's private key
	signedTx, err := types.SignTx(tx, types.NewLondonSigner(config.ChainId), senderPrivateKey)
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

// Send a DynamicFeeTx with default (suggested) values for GasFeeCap and GasTipCap
// If overPrice flag is true send an overpriced transaction to overbid previously failed
// transactions from this account
func sendDefaultDynamicTx(overPrice bool) (common.Hash, error) {
	// Set the gas fee cap and gas tip cap
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return common.Hash{}, err
	}

	gasTipCap, err := client.SuggestGasTipCap(context.Background())
	if err != nil {
		return common.Hash{}, err
	}

	// overprice transaction to outbid pending failed transaction from the account
	if overPrice {
		overPriceAmount := big.NewInt(10_000)
		gasPrice = big.NewInt(0).Add(gasPrice, overPriceAmount)
		gasTipCap = big.NewInt(0).Add(gasTipCap, overPriceAmount)

	}
	return sendDynamicFeeTx(gasPrice, gasTipCap)
}

// Send a DynamicFeeTx with GasFeeCap < GasTipCap
func sendSmallGasFeeCapDynamicFeeTx() (common.Hash, error) {
	gasTipCap, err := client.SuggestGasTipCap(context.Background())
	if err != nil {
		return common.Hash{}, err
	}
	// set gas price to half of gasTipCap so that it's smaller .
	gasPrice := big.NewInt(0).Quo(gasTipCap, big.NewInt(2))
	// Verify that indeed gasPrice < gasTipCap
	if gasPrice.Cmp(gasTipCap) >= 0 {
		return common.Hash{}, errors.New("gasPrice was expected to be less than gasTipCap in this test case")
	}
	return sendDynamicFeeTx(gasPrice, gasTipCap)
}

// Send a DynamicFeeTx with GasTipCap < GasFeeCap
func sendSmallGasTipCapDynamicFeeTx() (common.Hash, error) {
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return common.Hash{}, err
	}
	// set gas tip cap to half of gas price so that it's smaller.
	gasTipCap := big.NewInt(0).Quo(gasPrice, big.NewInt(2))
	// Verify that indeed gasTipCap < gasPrice
	if gasTipCap.Cmp(gasPrice) >= 0 {
		return common.Hash{}, errors.New("gasTipCap was expected to be less than gasPrice in this test case")
	}
	return sendDynamicFeeTx(gasPrice, gasTipCap)
}

// PRE-HERTZ TEST CASES

func testLegacyTxPreHertz() error {
	txHash, err := sendLegacyTransaction()
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

	// BaseFee should be nil pre-Hertz
	baseFee := block.BaseFee()
	if baseFee != nil {
		return fmt.Errorf("BaseFee is not nil at pre-Hertz block number %v", blockNr)
	}
	return nil
}

func testDefaultDynamicFeeTxPreHertz() error {
	_, err := sendDefaultDynamicTx(false)
	// DynamicFeeTx before Hertz should give ErrTxTypeNotSupported
	if err != nil && err.Error() != types.ErrTxTypeNotSupported.Error() {
		return fmt.Errorf("expected ErrTxTypeNotSupported but got '%v' instead", err)
	}
	return nil
}

func testSmallGasFeeCapDynamicFeeTxPreHertz() error {
	_, err := sendSmallGasFeeCapDynamicFeeTx()
	// DynamicFeeTx before Hertz should give ErrTxTypeNotSupported
	if err != nil && err.Error() != types.ErrTxTypeNotSupported.Error() {
		return fmt.Errorf("expected ErrTxTypeNotSupported but got '%v' instead", err)
	}
	return nil
}

func testSmallGasTipCapDynamicFeeTxPreHertz() error {
	_, err := sendSmallGasTipCapDynamicFeeTx()
	// DynamicFeeTx before Hertz should give ErrTxTypeNotSupported
	if err != nil && err.Error() != types.ErrTxTypeNotSupported.Error() {
		return fmt.Errorf("expected ErrTxTypeNotSupported but got '%v' instead", err)
	}
	if err == nil {
		return errors.New("error should not be nil")
	}
	return nil

}

func testSuggestedPricesPreHertz() error {
	// Get the suggested gas fee cap and gas tip cap
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return err
	}

	gasTipCap, err := client.SuggestGasTipCap(context.Background())
	if err != nil {
		return err
	}

	if gasPrice.Cmp(gasTipCap) != 0 {
		return errors.New("suggested gas price != suggested gasTipCap")
	}
	return nil

}

// POST-HERTZ TESTS

var testSuggestedPricesPostHertz = testSuggestedPricesPreHertz

func testLegacyTxPostHertz() error {
	txHash, err := sendLegacyTransaction()
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

	// BaseFee should be 0 post-Hertz
	baseFee := block.BaseFee()
	if baseFee == nil || baseFee.Cmp(common.Big0) != 0 {
		return fmt.Errorf("BaseFee is not 0 at post-Hertz block number %v", blockNr)
	}
	return nil
}

func testDefaultDynamicFeeTxPostHertz() error {
	txHash, err := sendDefaultDynamicTx(true)
	if err != nil {
		return err
	}
	receipt, err := utils.WaitForTransactionReceipt(client, txHash)
	if err != nil {
		return err
	}
	// Check receipt
	if receipt.Status != 1 {
		return fmt.Errorf("receipt.Status != 1. Receipt: %v", receipt)
	}

	tx, isPending, err := client.TransactionByHash(context.Background(), txHash)
	if isPending {
		return errors.New("transaction should not be pending")
	}
	if err != nil {
		return err
	}

	gasPrice := tx.GasPrice()
	gasFeeCap := tx.GasFeeCap()
	gasTipCap := tx.GasTipCap()

	// assert that gasPrice == gasFeeCap == gasTipCap
	if gasPrice.Cmp(gasFeeCap) != 0 || gasPrice.Cmp(gasTipCap) != 0 || gasTipCap.Cmp(gasFeeCap) != 0 {
		return fmt.Errorf("gasPrice, gasFeeCap and gasTip cap should all be equal when base fee is 0. gasPrice = %v , gasFeeCap = %v, gasTipCap =%v", gasPrice, gasFeeCap, gasTipCap)
	}

	blockNr := receipt.BlockNumber
	block, err := client.BlockByNumber(context.Background(), blockNr)
	if err != nil {
		return err
	}

	// BaseFee should be 0 post-Hertz
	baseFee := block.BaseFee()
	if baseFee == nil || baseFee.Cmp(common.Big0) != 0 {
		return fmt.Errorf("BaseFee is not 0 at post-Hertz block number %v", blockNr)
	}

	return nil
}

// Send a DynamicFeeTx with GasTipCap < GasFeeCap
func testSmallGasTipCapDynamicFeeTxPostHertz() error {
	txHash, err := sendSmallGasTipCapDynamicFeeTx()
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

	tx, isPending, err := client.TransactionByHash(context.Background(), txHash)
	if isPending {
		return errors.New("transaction should not be pending")
	}
	if err != nil {
		return err
	}

	gasPrice := tx.GasPrice()
	gasFeeCap := tx.GasFeeCap()
	gasTipCap := tx.GasTipCap()

	if gasTipCap.Cmp(gasFeeCap) >= 0 {
		return fmt.Errorf("gas tip cap should be less than gas fee cap in this test.  gasPrice=%v, gasFeeCap =%v, gasTipCap =%v", gasPrice, gasFeeCap, gasTipCap)
	}

	if gasPrice.Cmp(gasFeeCap) != 0 {
		return fmt.Errorf("gas price should be equal to gas fee cap. gasPrice=%v, gasFeeCap =%v, gasTipCap =%v", gasPrice, gasFeeCap, gasTipCap)
	}

	return nil
}

// Send a DynamicFeeTx with GasFeeCap < GasTipCap
func testSmallGasFeeCapDynamicFeeTxPostHertz() error {
	_, err := sendSmallGasFeeCapDynamicFeeTx()
	if err != nil && err.Error() != core.ErrTipAboveFeeCap.Error() {
		return fmt.Errorf("expected ErrTipAboveFeeCap but got '%v' instead", err)
	}
	if err == nil {
		return errors.New("error should not be nil")
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

func runPreHertzTests() {
	testCases := []TestCase{
		{
			name:               "testLegacyTxPreHertz",
			validationFunction: testLegacyTxPreHertz,
		},
		{
			name:               "testDefaultDynamicFeeTxPreHertz",
			validationFunction: testDefaultDynamicFeeTxPreHertz,
		},
		{
			name:               "testSmallGasFeeCapDynamicFeeTxPreHertz",
			validationFunction: testSmallGasFeeCapDynamicFeeTxPreHertz,
		},
		{
			name:               "testSmallGasTipCapDynamicFeeTxPreHertz",
			validationFunction: testSmallGasTipCapDynamicFeeTxPreHertz,
		},
		{
			name:               "testSuggestedPricesPreHertz",
			validationFunction: testSuggestedPricesPreHertz,
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
	runPreHertzTests()
	log.Println("All Pre-Hertz tests passed!")
}

func runPostHertzTests() {
	testCases := []TestCase{
		{
			name:               "testLegacyTxPostHertz",
			validationFunction: testLegacyTxPostHertz,
		},
		{
			name:               "testDefaultDynamicFeeTxPostHertz",
			validationFunction: testDefaultDynamicFeeTxPostHertz,
		},
		{
			name:               "testSmallGasTipCapDynamicFeeTxPostHertz",
			validationFunction: testSmallGasTipCapDynamicFeeTxPostHertz,
		},
		{
			name:               "testSmallGasFeeCapDynamicFeeTxPostHertz",
			validationFunction: testSmallGasFeeCapDynamicFeeTxPostHertz,
		},
		{
			name:               "testSuggestedPricesPostHertz",
			validationFunction: testSuggestedPricesPostHertz,
		},
	}
	runTestCasesSequentially(testCases)
}

func postHertzTests() {
	log.Println("Post-Hertz tests:")
	log.Printf("Waiting for block number %v to start running the test cases...\n", config.PostHertzBlockNumber)
	err := utils.WaitForBlockNumber(client, config.PostHertzBlockNumber)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Block number %v reached, running test cases....\n", config.PostHertzBlockNumber)
	runPostHertzTests()
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
