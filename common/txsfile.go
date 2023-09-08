package common

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"go.uber.org/zap"
)

// LoadTransactionCSVFiles loads transaction CSV files into a map[txHash]*TxEnvelope
// All transactions occurring in []knownTxsFiles are skipped
func LoadTransactionCSVFiles(log *zap.SugaredLogger, files, knownTxsFiles []string) (txs map[string]*TxEnvelope) { //nolint:gocognit
	// load previously known transaction hashes
	prevKnownTxs := LoadTxHashesFromMetadataCSVFiles(log, knownTxsFiles)

	cntProcessedFiles := 0
	txs = make(map[string]*TxEnvelope)
	for _, filename := range files {
		log.Infof("Loading %s ...", filename)
		cntProcessedFiles += 1
		cntTxInFileTotal := 0
		cntTxInFileNew := 0

		readFile, err := os.Open(filename)
		if err != nil {
			log.Errorw("os.Open", "error", err, "file", filename)
			return
		}
		defer readFile.Close()

		fileReader := bufio.NewReader(readFile)
		for {
			l, err := fileReader.ReadString('\n')
			if len(l) == 0 && err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				log.Errorw("fileReader.ReadString", "error", err)
				break
			}

			if len(l) < 66 {
				// log.Errorw("invalid line", "line", l)
				continue
			}

			l = strings.Trim(l, "\n")
			items := strings.Split(l, ",") // timestamp,hash,rlp
			if len(items) != 3 {
				log.Errorw("invalid line", "line", l)
				continue
			}

			cntTxInFileTotal += 1

			ts, err := strconv.Atoi(items[0])
			if err != nil {
				log.Errorw("strconv.Atoi", "error", err, "line", l)
				continue
			}
			txTimestamp := int64(ts)
			txHash := strings.ToLower(items[1])

			// Don't store transactions that were already seen previously (in refTxsFiles)
			if prevKnownTxs[txHash] {
				log.Debugf("Skipping tx that was already seen previously: %s", txHash)
				continue
			}

			// Dedupe transactions, and make sure to store the lowest timestamp
			if _, ok := txs[txHash]; ok {
				log.Debugf("Skipping duplicate tx: %s", txHash)

				if txTimestamp < txs[txHash].Summary.Timestamp {
					txs[txHash].Summary.Timestamp = txTimestamp
					log.Debugw("Updating timestamp for duplicate tx", "line", l)
				}

				continue
			}

			// Process this tx
			txSummary, _, err := parseTx(txTimestamp, txHash, items[2])
			if err != nil {
				log.Errorw("parseTx", "error", err, "line", l)
				continue
			}

			// Add to map
			txs[txHash] = &TxEnvelope{items[2], &txSummary}
			cntTxInFileNew += 1
		}
		log.Infow("Processed file",
			"txInFile", Printer.Sprintf("%d", cntTxInFileTotal),
			"txNew", Printer.Sprintf("%d", cntTxInFileNew),
			"txTotal", Printer.Sprintf("%d", len(txs)),
			"memUsedMiB", Printer.Sprintf("%d", GetMemUsageMb()),
		)
		// break
	}
	return txs
}

func parseTx(timestampMs int64, hash, rawTx string) (TxSummaryEntry, *types.Transaction, error) {
	tx, err := RLPStringToTx(rawTx)
	if err != nil {
		return TxSummaryEntry{}, nil, err
	}

	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		// fmt.Println("Error: ", err)
		_ = err
	}
	// prepare 'to' address
	to := ""
	if tx.To() != nil {
		to = tx.To().Hex()
	}

	// prepare '4 bytes' of data (function name)
	data4Bytes := ""
	if len(tx.Data()) >= 4 {
		data4Bytes = hexutil.Encode(tx.Data()[:4])
	}

	return TxSummaryEntry{
		Timestamp: timestampMs,
		Hash:      tx.Hash().Hex(),

		ChainID:   tx.ChainId().String(),
		From:      from.Hex(),
		To:        to,
		Value:     tx.Value().String(),
		Nonce:     fmt.Sprint(tx.Nonce()),
		Gas:       fmt.Sprint(tx.Gas()),
		GasPrice:  tx.GasPrice().String(),
		GasTipCap: tx.GasTipCap().String(),
		GasFeeCap: tx.GasFeeCap().String(),

		DataSize:   int64(len(tx.Data())),
		Data4Bytes: data4Bytes,
	}, tx, nil
}

// LoadTxHashesFromMetadataCSVFiles loads transaction hashes from metadata CSV files into a map[txHash]bool
func LoadTxHashesFromMetadataCSVFiles(log *zap.SugaredLogger, files []string) (txs map[string]bool) {
	txs = make(map[string]bool)
	for _, filename := range files {
		log.Infof("Loading %s ...", filename)

		readFile, err := os.Open(filename)
		if err != nil {
			log.Errorw("os.Open", "error", err, "file", filename)
			return
		}
		defer readFile.Close()
		csvReader := csv.NewReader(readFile)
		records, err := csvReader.ReadAll()
		if err != nil {
			log.Errorw("csvReader.ReadAll", "error", err, "file", filename)
			return
		}

		for _, record := range records {
			if len(record) < 2 {
				log.Errorw("invalid line", "line", record)
				continue
			}

			txHash := strings.ToLower(record[1])
			txs[txHash] = true
		}
	}

	return txs
}
