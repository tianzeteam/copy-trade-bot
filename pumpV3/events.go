package pumpV3

import (
	"encoding/base64"
	"fmt"
	"strings"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"
)

// 事件解码。六种交易指令（buy / sell / buy_exact_sol_in / buy_v2 / sell_v2 /
// buy_exact_quote_in_v2）共用同一个 TradeEvent 判别码，create / create_v2 共用
// CreateEvent 判别码，因此一套解码器同时覆盖新旧指令产生的事件。
//
// TradeEvent / CreateEvent 的解码器对缺失的尾部字段按零值处理，
// 升级前的旧事件可以直接解码。

const eventLogPrefix = "Program data: "

// DecodeEventsInLogs 仅从交易日志（"Program data: " 前缀）解码事件。
func DecodeEventsInLogs(logMessages []string) ([]any, error) {
	binaries, err := decodeEventsFromLogMessage(logMessages)
	if err != nil {
		return nil, err
	}
	return parseEvents(binaries), nil
}

// DecodeEventsInTransaction 从完整交易中解码事件，同时覆盖两种来源：
//  1. 日志消息（"Program data: " 前缀）
//  2. Anchor Self CPI（内部指令数据）
//
// programId 传 pump 程序地址（ProgramID）。
func DecodeEventsInTransaction(logMessages []string, innerInstructions []rpc.InnerInstruction, accountKeys solana.PublicKeySlice, programId solana.PublicKey) ([]any, error) {
	var binaries [][]byte

	logBinaries, err := decodeEventsFromLogMessage(logMessages)
	if err != nil {
		return nil, err
	}
	binaries = append(binaries, logBinaries...)

	cpiBinaries, err := decodeEventsFromEmitCPI(innerInstructions, accountKeys, programId)
	if err != nil {
		return nil, err
	}
	binaries = append(binaries, cpiBinaries...)

	return parseEvents(binaries), nil
}

func decodeEventsFromLogMessage(logMessages []string) ([][]byte, error) {
	var out [][]byte
	for _, log := range logMessages {
		if !strings.HasPrefix(log, eventLogPrefix) {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(log[len(eventLogPrefix):])
		if err != nil {
			return nil, fmt.Errorf("解码日志事件失败: %w", err)
		}
		out = append(out, raw)
	}
	return out, nil
}

func decodeEventsFromEmitCPI(innerInstructions []rpc.InnerInstruction, accountKeys solana.PublicKeySlice, targetProgramId solana.PublicKey) ([][]byte, error) {
	var out [][]byte
	for _, inner := range innerInstructions {
		for _, ix := range inner.Instructions {
			if int(ix.ProgramIDIndex) >= len(accountKeys) {
				continue
			}
			if !accountKeys[ix.ProgramIDIndex].Equals(targetProgramId) {
				continue
			}
			ixData, err := base58.Decode(ix.Data.String())
			if err != nil {
				return nil, fmt.Errorf("解码 CPI 指令数据失败: %w", err)
			}
			// Self CPI 的事件数据同样带 8 字节判别码前缀。
			if len(ixData) < 8 {
				continue
			}
			out = append(out, ixData)
		}
	}
	return out, nil
}

// parseEvents 逐个解析事件二进制。未知判别码或解析失败的数据直接跳过：
// 内部指令列表里可能混有非事件数据，不能因为一条坏数据丢掉整个交易。
func parseEvents(binaries [][]byte) []any {
	var evts []any
	for _, raw := range binaries {
		if len(raw) < 8 {
			continue
		}
		evt, err := ParseAnyEvent(raw)
		if err != nil {
			continue
		}
		evts = append(evts, evt)
	}
	return evts
}

// ParseAnyEvent 按判别码分发并解码单个事件。
// 只解码跟单流程关心的四类事件；其余事件返回错误由调用方跳过。
func ParseAnyEvent(eventData []byte) (any, error) {
	if len(eventData) < 8 {
		return nil, fmt.Errorf("事件数据过短: %d 字节", len(eventData))
	}

	var disc [8]byte
	copy(disc[:], eventData[:8])
	dec := bin.NewBorshDecoder(eventData[8:])

	switch disc {
	case EventTradeEvent:
		value := new(TradeEvent)
		if err := value.UnmarshalWithDecoder(dec); err != nil {
			return nil, fmt.Errorf("解码 TradeEvent 失败: %w", err)
		}
		return value, nil
	case EventCreateEvent:
		value := new(CreateEvent)
		if err := value.UnmarshalWithDecoder(dec); err != nil {
			return nil, fmt.Errorf("解码 CreateEvent 失败: %w", err)
		}
		return value, nil
	case EventCompleteEvent:
		value := new(CompleteEvent)
		if err := value.UnmarshalWithDecoder(dec); err != nil {
			return nil, fmt.Errorf("解码 CompleteEvent 失败: %w", err)
		}
		return value, nil
	case EventCompletePumpAmmMigration:
		value := new(CompletePumpAmmMigrationEvent)
		if err := value.UnmarshalWithDecoder(dec); err != nil {
			return nil, fmt.Errorf("解码 CompletePumpAmmMigrationEvent 失败: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("未知事件判别码: %x", disc)
	}
}
