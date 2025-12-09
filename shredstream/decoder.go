package shredstream

import (
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

// DecodeEntries 解码 bincode 序列化的 Vec<Entry>
// 使用 solana-go 的 binary 解码器
func DecodeEntries(data []byte) ([]Entry, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("data too short for vec length")
	}

	decoder := bin.NewBinDecoder(data)

	// 读取 Vec 长度 (u64 little-endian)
	vecLen, err := decoder.ReadUint64(bin.LE)
	if err != nil {
		return nil, fmt.Errorf("read vec length: %w", err)
	}

	// 合理性检查
	if vecLen > 10000 {
		return nil, fmt.Errorf("vec length too large: %d", vecLen)
	}

	entries := make([]Entry, 0, vecLen)
	for i := uint64(0); i < vecLen; i++ {
		entry, err := decodeEntry(decoder)
		if err != nil {
			return entries, fmt.Errorf("decode entry %d: %w", i, err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// decodeEntry 解码单个 Entry
func decodeEntry(decoder *bin.Decoder) (Entry, error) {
	var entry Entry

	// 1. num_hashes (u64)
	numHashes, err := decoder.ReadUint64(bin.LE)
	if err != nil {
		return entry, fmt.Errorf("read num_hashes: %w", err)
	}
	entry.NumHashes = numHashes

	// 2. hash (32 bytes)
	hashBytes, err := decoder.ReadBytes(32)
	if err != nil {
		return entry, fmt.Errorf("read hash: %w", err)
	}
	entry.Hash = solana.HashFromBytes(hashBytes)

	// 3. transactions (Vec<Transaction>) - 使用 solana-go 的解码器
	txCount, err := decoder.ReadUint64(bin.LE)
	if err != nil {
		return entry, fmt.Errorf("read tx count: %w", err)
	}

	// 合理性检查
	if txCount > 2048 {
		return entry, fmt.Errorf("tx count too large: %d", txCount)
	}

	entry.Transactions = make([]solana.Transaction, 0, txCount)
	for j := uint64(0); j < txCount; j++ {
		var tx solana.Transaction
		// 使用 solana-go 内置的 Transaction 解码
		if err := decoder.Decode(&tx); err != nil {
			return entry, fmt.Errorf("decode transaction %d: %w", j, err)
		}
		entry.Transactions = append(entry.Transactions, tx)
	}

	return entry, nil
}
