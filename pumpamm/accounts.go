package pumpamm

import (
	"fmt"
	ag_binary "github.com/gagliardetto/binary"
	ag_solanago "github.com/gagliardetto/solana-go"
)

type GlobalConfigAccount struct {
	// The admin pubkey
	Admin ag_solanago.PublicKey

	// The lp fee in basis points (0.01%)
	LpFeeBasisPoints uint64

	// The protocol fee in basis points (0.01%)
	ProtocolFeeBasisPoints uint64

	// Flags to disable certain functionality
	// bit 0 - Disable create pool
	// bit 1 - Disable deposit
	// bit 2 - Disable withdraw
	// bit 3 - Disable buy
	// bit 4 - Disable sell
	DisableFlags uint8

	// Addresses of the protocol fee recipients
	ProtocolFeeRecipients [8]ag_solanago.PublicKey
}

var GlobalConfigAccountDiscriminator = [8]byte{149, 8, 156, 202, 160, 252, 176, 217}

func (obj GlobalConfigAccount) MarshalWithEncoder(encoder *ag_binary.Encoder) (err error) {
	// Write account discriminator:
	err = encoder.WriteBytes(GlobalConfigAccountDiscriminator[:], false)
	if err != nil {
		return err
	}
	// Serialize `Admin` param:
	err = encoder.Encode(obj.Admin)
	if err != nil {
		return err
	}
	// Serialize `LpFeeBasisPoints` param:
	err = encoder.Encode(obj.LpFeeBasisPoints)
	if err != nil {
		return err
	}
	// Serialize `ProtocolFeeBasisPoints` param:
	err = encoder.Encode(obj.ProtocolFeeBasisPoints)
	if err != nil {
		return err
	}
	// Serialize `DisableFlags` param:
	err = encoder.Encode(obj.DisableFlags)
	if err != nil {
		return err
	}
	// Serialize `ProtocolFeeRecipients` param:
	err = encoder.Encode(obj.ProtocolFeeRecipients)
	if err != nil {
		return err
	}
	return nil
}

func (obj *GlobalConfigAccount) UnmarshalWithDecoder(decoder *ag_binary.Decoder) (err error) {
	// Read and check account discriminator:
	{
		discriminator, err := decoder.ReadTypeID()
		if err != nil {
			return err
		}
		if !discriminator.Equal(GlobalConfigAccountDiscriminator[:]) {
			return fmt.Errorf(
				"wrong discriminator: wanted %s, got %s",
				"[149 8 156 202 160 252 176 217]",
				fmt.Sprint(discriminator[:]))
		}
	}
	// Deserialize `Admin`:
	err = decoder.Decode(&obj.Admin)
	if err != nil {
		return err
	}
	// Deserialize `LpFeeBasisPoints`:
	err = decoder.Decode(&obj.LpFeeBasisPoints)
	if err != nil {
		return err
	}
	// Deserialize `ProtocolFeeBasisPoints`:
	err = decoder.Decode(&obj.ProtocolFeeBasisPoints)
	if err != nil {
		return err
	}
	// Deserialize `DisableFlags`:
	err = decoder.Decode(&obj.DisableFlags)
	if err != nil {
		return err
	}
	// Deserialize `ProtocolFeeRecipients`:
	err = decoder.Decode(&obj.ProtocolFeeRecipients)
	if err != nil {
		return err
	}
	return nil
}

type PoolAccount struct {
	PoolBump              uint8
	Index                 uint16
	Creator               ag_solanago.PublicKey
	BaseMint              ag_solanago.PublicKey
	QuoteMint             ag_solanago.PublicKey
	LpMint                ag_solanago.PublicKey
	PoolBaseTokenAccount  ag_solanago.PublicKey
	PoolQuoteTokenAccount ag_solanago.PublicKey

	// True circulating supply without burns and lock-ups
	LpSupply uint64
}

var PoolAccountDiscriminator = [8]byte{241, 154, 109, 4, 17, 177, 109, 188}

func (obj PoolAccount) MarshalWithEncoder(encoder *ag_binary.Encoder) (err error) {
	// Write account discriminator:
	err = encoder.WriteBytes(PoolAccountDiscriminator[:], false)
	if err != nil {
		return err
	}
	// Serialize `PoolBump` param:
	err = encoder.Encode(obj.PoolBump)
	if err != nil {
		return err
	}
	// Serialize `Index` param:
	err = encoder.Encode(obj.Index)
	if err != nil {
		return err
	}
	// Serialize `Creator` param:
	err = encoder.Encode(obj.Creator)
	if err != nil {
		return err
	}
	// Serialize `BaseMint` param:
	err = encoder.Encode(obj.BaseMint)
	if err != nil {
		return err
	}
	// Serialize `QuoteMint` param:
	err = encoder.Encode(obj.QuoteMint)
	if err != nil {
		return err
	}
	// Serialize `LpMint` param:
	err = encoder.Encode(obj.LpMint)
	if err != nil {
		return err
	}
	// Serialize `PoolBaseTokenAccount` param:
	err = encoder.Encode(obj.PoolBaseTokenAccount)
	if err != nil {
		return err
	}
	// Serialize `PoolQuoteTokenAccount` param:
	err = encoder.Encode(obj.PoolQuoteTokenAccount)
	if err != nil {
		return err
	}
	// Serialize `LpSupply` param:
	err = encoder.Encode(obj.LpSupply)
	if err != nil {
		return err
	}
	return nil
}

func (obj *PoolAccount) UnmarshalWithDecoder(decoder *ag_binary.Decoder) (err error) {
	// Read and check account discriminator:
	{
		discriminator, err := decoder.ReadTypeID()
		if err != nil {
			return err
		}
		if !discriminator.Equal(PoolAccountDiscriminator[:]) {
			return fmt.Errorf(
				"wrong discriminator: wanted %s, got %s",
				"[241 154 109 4 17 177 109 188]",
				fmt.Sprint(discriminator[:]))
		}
	}
	// Deserialize `PoolBump`:
	err = decoder.Decode(&obj.PoolBump)
	if err != nil {
		return err
	}
	// Deserialize `Index`:
	err = decoder.Decode(&obj.Index)
	if err != nil {
		return err
	}
	// Deserialize `Creator`:
	err = decoder.Decode(&obj.Creator)
	if err != nil {
		return err
	}
	// Deserialize `BaseMint`:
	err = decoder.Decode(&obj.BaseMint)
	if err != nil {
		return err
	}
	// Deserialize `QuoteMint`:
	err = decoder.Decode(&obj.QuoteMint)
	if err != nil {
		return err
	}
	// Deserialize `LpMint`:
	err = decoder.Decode(&obj.LpMint)
	if err != nil {
		return err
	}
	// Deserialize `PoolBaseTokenAccount`:
	err = decoder.Decode(&obj.PoolBaseTokenAccount)
	if err != nil {
		return err
	}
	// Deserialize `PoolQuoteTokenAccount`:
	err = decoder.Decode(&obj.PoolQuoteTokenAccount)
	if err != nil {
		return err
	}
	// Deserialize `LpSupply`:
	err = decoder.Decode(&obj.LpSupply)
	if err != nil {
		return err
	}
	return nil
}
