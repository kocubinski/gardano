package address

import (
	"crypto/ed25519"
	"fmt"

	"github.com/gcash/bchutil/bech32"
	"golang.org/x/crypto/blake2b"
)

const Blake2b224Len = 28

type Address []byte

func (addr Address) String() string {
	addr5Bit, err := bech32.ConvertBits(addr, 8, 5, true)
	if err != nil {
		panic(err)
	}
	res, err := bech32.Encode("addr", addr5Bit)
	if err != nil {
		panic(err)
	}
	return res
}

func FromBech32(addrBech32 string) (Address, error) {
	hrp, data, err := bech32.Decode(addrBech32)
	if err != nil {
		return nil, err
	}
	if hrp != "addr" {
		return nil, fmt.Errorf("invalid hrp: %s", hrp)
	}
	addrBz, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return nil, err
	}
	return Address(addrBz), nil
}

func MustFromBech32(addrBech32 string) Address {
	addr, err := FromBech32(addrBech32)
	if err != nil {
		panic(err)
	}
	return addr
}

func Blake2b224(data []byte) (result [Blake2b224Len]byte, err error) {
	b2b, err := blake2b.New(Blake2b224Len, nil)
	if err != nil {
		err = fmt.Errorf("error blake2b224 init: %w", err)
		return
	}
	b2b.Write(data)
	copy(result[:], b2b.Sum(nil)[:Blake2b224Len])
	return
}

func NewMainnetPaymentOnlyFromPubkey(pub []byte) (Address, error) {
	// see CIP-19 for header explanation. This header encodes the following:
	// - 4 MSBs: payment only address
	// - 4 LSBs: mainnet
	header := byte(0b01100001)
	addr := []byte{header}
	keyHash, err := Blake2b224(pub)
	if err != nil {
		return nil, err
	}
	addr = append(addr, keyHash[:]...)
	return Address(addr[:]), err
}

func PrivateKeyFromBech32(privBech32 string) (ed25519.PrivateKey, error) {
	hrp, data, err := bech32.Decode(privBech32)
	if err != nil {
		return nil, err
	}
	if hrp != "addr_sk" {
		return nil, fmt.Errorf("invalid hrp: %s", hrp)
	}
	privBz, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return nil, err
	}
	return ed25519.NewKeyFromSeed(privBz), nil
}
