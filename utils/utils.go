package utils

import (
	"context"
	"errors"
	"time"

	"log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func WaitForBlockNumber(client *ethclient.Client, blockNumber uint64) error {
	for {
		currentBlockNumber, err := client.BlockNumber(context.Background())
		if err != nil {
			return err
		}

		// Target blockNumber reached
		if currentBlockNumber >= blockNumber {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
}

func WaitForTransactionReceipt(client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute) // Timeout after 1 minute
	log.Println("Waiting for transaction receipt")
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("Timeout exceeded, transaction receipt not found")
		default:
			receipt, err := client.TransactionReceipt(context.Background(), txHash)
			if receipt == nil || err != nil {
				time.Sleep(5 * time.Second) // Wait for 5 seconds before polling again
			} else {
				return receipt, nil
			}
		}
	}
}
