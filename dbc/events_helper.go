package dbc

import (
	"encoding/base64"
	"fmt"

	ag_solanago "github.com/gagliardetto/solana-go"
	ag_rpc "github.com/gagliardetto/solana-go/rpc"
	ag_base58 "github.com/mr-tron/base58"
)

// DecodeEventsFromInnerInstructions 从内部指令中解码 DBC 事件
// 这个函数用于从 Yellowstone gRPC 的 InnerInstructions 中提取事件数据
func DecodeEventsFromInnerInstructions(innerInstructions []ag_rpc.InnerInstruction, accountKeys ag_solanago.PublicKeySlice, targetProgramId ag_solanago.PublicKey) (eventBinaries [][]byte, err error) {
	for _, parsedIx := range innerInstructions {
		for _, ix := range parsedIx.Instructions {
			if accountKeys[ix.ProgramIDIndex] != targetProgramId {
				continue
			}

			var ixData []byte
			if ixData, err = ag_base58.Decode(ix.Data.String()); err != nil {
				return
			}
			if len(ixData) < 8 {
				continue
			}

			// 跳过前8个字节的判别器，剩余的是事件数据
			eventBase64 := base64.StdEncoding.EncodeToString(ixData[8:])
			var eventBinary []byte
			if eventBinary, err = base64.StdEncoding.DecodeString(eventBase64); err != nil {
				return
			}
			eventBinaries = append(eventBinaries, eventBinary)
		}
	}
	return
}

// ParseEventsFromBinaries 从二进制数据中解析 DBC 事件
func ParseEventsFromBinaries(eventBinaries [][]byte) (events []interface{}, err error) {
	for _, eventBinary := range eventBinaries {
		if len(eventBinary) < 8 {
			continue
		}

		// 使用 ParseAnyEvent 来解析事件
		event, parseErr := ParseAnyEvent(eventBinary)
		if parseErr != nil {
			// 继续处理其他事件，不中断整个流程
			continue
		}

		events = append(events, event)
	}

	return events, nil
}

// DecodeAndParseEvents 一站式解码和解析 DBC 事件
func DecodeAndParseEvents(innerInstructions []ag_rpc.InnerInstruction, accountKeys ag_solanago.PublicKeySlice, targetProgramId ag_solanago.PublicKey) (events []interface{}, err error) {
	eventBinaries, err := DecodeEventsFromInnerInstructions(innerInstructions, accountKeys, targetProgramId)
	if err != nil {
		return nil, fmt.Errorf("failed to decode events from inner instructions: %w", err)
	}

	events, err = ParseEventsFromBinaries(eventBinaries)
	if err != nil {
		return nil, fmt.Errorf("failed to parse events from binaries: %w", err)
	}

	return events, nil
}
