package pumpamm

import "crypto/sha256"

var BuyDiscriminator = CalculateDiscriminator("global:buy")
var SellDiscriminator = CalculateDiscriminator("global:sell")
var CreateDiscriminator = CalculateDiscriminator("global:create")

var AnchorSelfCPILogDiscriminator = [8]uint8{228, 69, 165, 46, 81, 203, 154, 29}
var AnchorSelfCPILogSwapDiscriminator = [8]uint8{189, 219, 127, 211, 78, 230, 97, 238}

func CalculateDiscriminator(instructionName string) [8]byte {
	hash := sha256.Sum256([]byte(instructionName))
	var discriminator [8]byte
	copy(discriminator[:], hash[:8])
	return discriminator
}
