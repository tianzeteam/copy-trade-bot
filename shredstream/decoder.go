package shredstream

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

// DecodeEntries 解码 bincode 序列化的 Vec<Entry>
func DecodeEntries(data []byte) ([]Entry, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("data too short for vec length")
	}

	reader := bytes.NewReader(data)

	// 读取 Vec 长度 (u64 little-endian)
	var vecLen uint64
	if err := binary.Read(reader, binary.LittleEndian, &vecLen); err != nil {
		return nil, fmt.Errorf("read vec length: %w", err)
	}

	// 合理性检查
	if vecLen > 10000 {
		return nil, fmt.Errorf("vec length too large: %d", vecLen)
	}

	entries := make([]Entry, 0, vecLen)
	for i := uint64(0); i < vecLen; i++ {
		entry, err := decodeEntry(reader)
		if err != nil {
			return entries, fmt.Errorf("decode entry %d: %w", i, err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// decodeEntry 解码单个 Entry
func decodeEntry(reader *bytes.Reader) (Entry, error) {
	var entry Entry

	// 1. num_hashes (u64)
	if err := binary.Read(reader, binary.LittleEndian, &entry.NumHashes); err != nil {
		return entry, fmt.Errorf("read num_hashes: %w", err)
	}

	// 2. hash (32 bytes)
	hashBytes := make([]byte, 32)
	if _, err := reader.Read(hashBytes); err != nil {
		return entry, fmt.Errorf("read hash: %w", err)
	}
	entry.Hash = solana.HashFromBytes(hashBytes)

	// 3. transactions (Vec<Transaction>)
	var txCount uint64
	if err := binary.Read(reader, binary.LittleEndian, &txCount); err != nil {
		return entry, fmt.Errorf("read tx count: %w", err)
	}

	// 合理性检查
	if txCount > 1000 {
		return entry, fmt.Errorf("tx count too large: %d", txCount)
	}

	entry.Transactions = make([]solana.Transaction, 0, txCount)
	for j := uint64(0); j < txCount; j++ {
		tx, err := decodeTransaction(reader)
		if err != nil {
			return entry, fmt.Errorf("decode transaction %d: %w", j, err)
		}
		entry.Transactions = append(entry.Transactions, tx)
	}

	return entry, nil
}

// decodeTransaction 解码单个 Transaction (bincode 格式)
func decodeTransaction(reader *bytes.Reader) (solana.Transaction, error) {
	var tx solana.Transaction

	// 1. 读取签名数量 (compact-u16 或 u64，取决于序列化格式)
	sigCount, err := readCompactU16(reader)
	if err != nil {
		return tx, fmt.Errorf("read signature count: %w", err)
	}

	if sigCount > 256 {
		return tx, fmt.Errorf("signature count too large: %d", sigCount)
	}

	// 2. 读取签名
	tx.Signatures = make([]solana.Signature, sigCount)
	for i := range sigCount {
		sigBytes := make([]byte, 64)
		if _, err := reader.Read(sigBytes); err != nil {
			return tx, fmt.Errorf("read signature %d: %w", i, err)
		}
		copy(tx.Signatures[i][:], sigBytes)
	}

	// 3. 读取 Message
	msg, err := decodeMessage(reader)
	if err != nil {
		return tx, fmt.Errorf("decode message: %w", err)
	}
	tx.Message = msg

	return tx, nil
}

// decodeMessage 解码 Message
func decodeMessage(reader *bytes.Reader) (solana.Message, error) {
	var msg solana.Message

	// 1. Header (3 bytes)
	header := make([]byte, 3)
	if _, err := reader.Read(header); err != nil {
		return msg, fmt.Errorf("read header: %w", err)
	}
	msg.Header.NumRequiredSignatures = header[0]
	msg.Header.NumReadonlySignedAccounts = header[1]
	msg.Header.NumReadonlyUnsignedAccounts = header[2]

	// 2. Account Keys (compact-u16 + pubkeys)
	keyCount, err := readCompactU16(reader)
	if err != nil {
		return msg, fmt.Errorf("read key count: %w", err)
	}

	if keyCount > 256 {
		return msg, fmt.Errorf("key count too large: %d", keyCount)
	}

	msg.AccountKeys = make([]solana.PublicKey, keyCount)
	for i := uint16(0); i < keyCount; i++ {
		keyBytes := make([]byte, 32)
		if _, err := reader.Read(keyBytes); err != nil {
			return msg, fmt.Errorf("read account key %d: %w", i, err)
		}
		msg.AccountKeys[i] = solana.PublicKeyFromBytes(keyBytes)
	}

	// 3. Recent Blockhash (32 bytes)
	hashBytes := make([]byte, 32)
	if _, err := reader.Read(hashBytes); err != nil {
		return msg, fmt.Errorf("read recent blockhash: %w", err)
	}
	msg.RecentBlockhash = solana.HashFromBytes(hashBytes)

	// 4. Instructions (compact-u16 + instructions)
	ixCount, err := readCompactU16(reader)
	if err != nil {
		return msg, fmt.Errorf("read instruction count: %w", err)
	}

	if ixCount > 256 {
		return msg, fmt.Errorf("instruction count too large: %d", ixCount)
	}

	msg.Instructions = make([]solana.CompiledInstruction, ixCount)
	for i := uint16(0); i < ixCount; i++ {
		ix, err := decodeCompiledInstruction(reader)
		if err != nil {
			return msg, fmt.Errorf("decode instruction %d: %w", i, err)
		}
		msg.Instructions[i] = ix
	}

	return msg, nil
}

// decodeCompiledInstruction 解码 CompiledInstruction
func decodeCompiledInstruction(reader *bytes.Reader) (solana.CompiledInstruction, error) {
	var ix solana.CompiledInstruction

	// 1. Program ID Index (u8)
	programIDIndex, err := reader.ReadByte()
	if err != nil {
		return ix, fmt.Errorf("read program id index: %w", err)
	}
	ix.ProgramIDIndex = uint16(programIDIndex)

	// 2. Account Indexes (compact-u16 + bytes)
	accountCount, err := readCompactU16(reader)
	if err != nil {
		return ix, fmt.Errorf("read account count: %w", err)
	}

	ix.Accounts = make([]uint16, accountCount)
	for i := uint16(0); i < accountCount; i++ {
		accountIdx, err := reader.ReadByte()
		if err != nil {
			return ix, fmt.Errorf("read account index %d: %w", i, err)
		}
		ix.Accounts[i] = uint16(accountIdx)
	}

	// 3. Data (compact-u16 + bytes)
	dataLen, err := readCompactU16(reader)
	if err != nil {
		return ix, fmt.Errorf("read data length: %w", err)
	}

	ix.Data = make([]byte, dataLen)
	if _, err := reader.Read(ix.Data); err != nil {
		return ix, fmt.Errorf("read data: %w", err)
	}

	return ix, nil
}

// readCompactU16 读取 Solana compact-u16 (ShortU16) 编码的整数
// 编码规则 (参考 solana-short-vec crate):
// - 1 byte  : 0xxxxxxx                   => 00000000 0xxxxxxx :      0 -    127
// - 2 bytes : 1xxxxxxx 0yyyyyyy          => 00yyyyyy yxxxxxxx :    128 - 16,383
// - 3 bytes : 1xxxxxxx 1yyyyyyy 000000zz => zzyyyyyy yxxxxxxx : 16,384 - 65,535
//
// 每个字节的最高位 (0x80) 是 continuation bit:
// - 0 表示这是最后一个字节
// - 1 表示还有更多字节
func readCompactU16(reader *bytes.Reader) (uint16, error) {
	var val uint16 = 0

	for i := 0; i < 3; i++ {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}

		// 提取低 7 位并左移
		val |= uint16(b&0x7f) << (i * 7)

		// 检查 continuation bit
		if b&0x80 == 0 {
			return val, nil
		}
	}

	return 0, fmt.Errorf("compact-u16 encoding too long (> 3 bytes)")
}
