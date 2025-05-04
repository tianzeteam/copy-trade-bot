package pumpamm

import (
	"encoding/json"
	"errors"
	"fmt"
	ag_jsonrpc "github.com/gagliardetto/solana-go/rpc/jsonrpc"
)

var (
	_ *json.Encoder        = nil
	_ *ag_jsonrpc.RPCError = nil
	_ fmt.Formatter        = nil
	_                      = errors.ErrUnsupported
)
var (
	ErrFeeBasisPointsExceedsMaximum = &customErrorDef{
		code: 6000,
		msg:  "",
		name: "FeeBasisPointsExceedsMaximum",
	}
	ErrZeroBaseAmount = &customErrorDef{
		code: 6001,
		msg:  "",
		name: "ZeroBaseAmount",
	}
	ErrZeroQuoteAmount = &customErrorDef{
		code: 6002,
		msg:  "",
		name: "ZeroQuoteAmount",
	}
	ErrTooLittlePoolTokenLiquidity = &customErrorDef{
		code: 6003,
		msg:  "",
		name: "TooLittlePoolTokenLiquidity",
	}
	ErrExceededSlippage = &customErrorDef{
		code: 6004,
		msg:  "",
		name: "ExceededSlippage",
	}
	ErrInvalidAdmin = &customErrorDef{
		code: 6005,
		msg:  "",
		name: "InvalidAdmin",
	}
	ErrUnsupportedBaseMint = &customErrorDef{
		code: 6006,
		msg:  "",
		name: "UnsupportedBaseMint",
	}
	ErrUnsupportedQuoteMint = &customErrorDef{
		code: 6007,
		msg:  "",
		name: "UnsupportedQuoteMint",
	}
	ErrInvalidBaseMint = &customErrorDef{
		code: 6008,
		msg:  "",
		name: "InvalidBaseMint",
	}
	ErrInvalidQuoteMint = &customErrorDef{
		code: 6009,
		msg:  "",
		name: "InvalidQuoteMint",
	}
	ErrInvalidLpMint = &customErrorDef{
		code: 6010,
		msg:  "",
		name: "InvalidLpMint",
	}
	ErrAllProtocolFeeRecipientsShouldBeNonZero = &customErrorDef{
		code: 6011,
		msg:  "",
		name: "AllProtocolFeeRecipientsShouldBeNonZero",
	}
	ErrUnsortedNotUniqueProtocolFeeRecipients = &customErrorDef{
		code: 6012,
		msg:  "",
		name: "UnsortedNotUniqueProtocolFeeRecipients",
	}
	ErrInvalidProtocolFeeRecipient = &customErrorDef{
		code: 6013,
		msg:  "",
		name: "InvalidProtocolFeeRecipient",
	}
	ErrInvalidPoolBaseTokenAccount = &customErrorDef{
		code: 6014,
		msg:  "",
		name: "InvalidPoolBaseTokenAccount",
	}
	ErrInvalidPoolQuoteTokenAccount = &customErrorDef{
		code: 6015,
		msg:  "",
		name: "InvalidPoolQuoteTokenAccount",
	}
	ErrBuyMoreBaseAmountThanPoolReserves = &customErrorDef{
		code: 6016,
		msg:  "",
		name: "BuyMoreBaseAmountThanPoolReserves",
	}
	ErrDisabledCreatePool = &customErrorDef{
		code: 6017,
		msg:  "",
		name: "DisabledCreatePool",
	}
	ErrDisabledDeposit = &customErrorDef{
		code: 6018,
		msg:  "",
		name: "DisabledDeposit",
	}
	ErrDisabledWithdraw = &customErrorDef{
		code: 6019,
		msg:  "",
		name: "DisabledWithdraw",
	}
	ErrDisabledBuy = &customErrorDef{
		code: 6020,
		msg:  "",
		name: "DisabledBuy",
	}
	ErrDisabledSell = &customErrorDef{
		code: 6021,
		msg:  "",
		name: "DisabledSell",
	}
	ErrSameMint = &customErrorDef{
		code: 6022,
		msg:  "",
		name: "SameMint",
	}
	ErrOverflow = &customErrorDef{
		code: 6023,
		msg:  "",
		name: "Overflow",
	}
	ErrTruncation = &customErrorDef{
		code: 6024,
		msg:  "",
		name: "Truncation",
	}
	ErrDivisionByZero = &customErrorDef{
		code: 6025,
		msg:  "",
		name: "DivisionByZero",
	}
	ErrNewSizeLessThanCurrentSize = &customErrorDef{
		code: 6026,
		msg:  "",
		name: "NewSizeLessThanCurrentSize",
	}
	ErrAccountTypeNotSupported = &customErrorDef{
		code: 6027,
		msg:  "",
		name: "AccountTypeNotSupported",
	}
	Errors = map[int]CustomError{
		6000: ErrFeeBasisPointsExceedsMaximum,
		6001: ErrZeroBaseAmount,
		6002: ErrZeroQuoteAmount,
		6003: ErrTooLittlePoolTokenLiquidity,
		6004: ErrExceededSlippage,
		6005: ErrInvalidAdmin,
		6006: ErrUnsupportedBaseMint,
		6007: ErrUnsupportedQuoteMint,
		6008: ErrInvalidBaseMint,
		6009: ErrInvalidQuoteMint,
		6010: ErrInvalidLpMint,
		6011: ErrAllProtocolFeeRecipientsShouldBeNonZero,
		6012: ErrUnsortedNotUniqueProtocolFeeRecipients,
		6013: ErrInvalidProtocolFeeRecipient,
		6014: ErrInvalidPoolBaseTokenAccount,
		6015: ErrInvalidPoolQuoteTokenAccount,
		6016: ErrBuyMoreBaseAmountThanPoolReserves,
		6017: ErrDisabledCreatePool,
		6018: ErrDisabledDeposit,
		6019: ErrDisabledWithdraw,
		6020: ErrDisabledBuy,
		6021: ErrDisabledSell,
		6022: ErrSameMint,
		6023: ErrOverflow,
		6024: ErrTruncation,
		6025: ErrDivisionByZero,
		6026: ErrNewSizeLessThanCurrentSize,
		6027: ErrAccountTypeNotSupported,
	}
)

type CustomError interface {
	Code() int
	Name() string
	Error() string
}

type customErrorDef struct {
	code int
	name string
	msg  string
}

func (e *customErrorDef) Code() int {
	return e.code
}

func (e *customErrorDef) Name() string {
	return e.name
}

func (e *customErrorDef) Error() string {
	return fmt.Sprintf("%s(%d): %s", e.name, e.code, e.msg)
}

func DecodeCustomError(rpcErr error) (err error, ok bool) {
	if errCode, o := decodeErrorCode(rpcErr); o {
		if customErr, o := Errors[errCode]; o {
			err = customErr
			ok = true
			return
		}
	}
	return
}

func decodeErrorCode(rpcErr error) (errorCode int, ok bool) {
	var jErr *ag_jsonrpc.RPCError
	if errors.As(rpcErr, &jErr) && jErr.Data != nil {
		if root, o := jErr.Data.(map[string]interface{}); o {
			if rootErr, o := root["err"].(map[string]interface{}); o {
				if rootErrInstructionError, o := rootErr["InstructionError"]; o {
					if rootErrInstructionErrorItems, o := rootErrInstructionError.([]interface{}); o {
						if len(rootErrInstructionErrorItems) == 2 {
							if v, o := rootErrInstructionErrorItems[1].(map[string]interface{}); o {
								if v2, o := v["Custom"].(json.Number); o {
									if code, err := v2.Int64(); err == nil {
										ok = true
										errorCode = int(code)
									}
								} else if v2, o := v["Custom"].(float64); o {
									ok = true
									errorCode = int(v2)
								}
							}
						}
					}
				}
			}
		}
	}
	return
}
