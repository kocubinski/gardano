package address

import (
	"fmt"

	"github.com/gcash/bchutil/bech32"
	"golang.org/x/crypto/blake2b"
)

const Blake2b224Len = 28

type Address string

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

func NewMainnetPaymentOnly(pub []byte) (Address, error) {
	header := byte(0b01100001)
	addr := []byte{header}
	keyHash, err := Blake2b224(pub)
	if err != nil {
		return "", err
	}
	addr = append(addr, keyHash[:]...)
	addr5Bit, err := bech32.ConvertBits(addr, 8, 5, true)
	if err != nil {
		return "", err
	}
	res, err := bech32.Encode("addr", addr5Bit)
	return Address(res), err
}
