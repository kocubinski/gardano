package main_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"testing"

	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/gcash/bchutil/bech32"
	"github.com/kocubinski/go-cardano/address"
	"github.com/stretchr/testify/require"
)

const (
	skey1Cbor   = "5820816ae1825fd44eec487b0d8b0de63eb85e1f15ea9fd71ffef6bdf0291197406d"
	vkey1Cbor   = "5820291ad90a6a5369be7b78f08810f64c75c8070976a638baf0aabe3960b8acb773"
	skey2Bech32 = "addr_sk1p7m6rndxn0s097zn4ga628dtm6zakep8x62vfnlnmpj3v9feey4shy5xx7"
	vkey2Bech32 = "addr_vk1rmvagkuqm0zqjmrw3p8m6k6ejc38zj5x68lfgzln4vwurc0gmdnqqhy5ad"
	addrBech32  = "addr1vx64czye08j6pkysk3fyfm90jz2tyft5tzjktz5h22wkn2gqhw2sa"
)

func TestKeys_CBOR(t *testing.T) {
	cborBz, err := hex.DecodeString(skey1Cbor)
	require.NoError(t, err)
	var bz []byte
	_, err = cbor.Decode(cborBz, &bz)
	require.NoError(t, err)
	priv := ed25519.NewKeyFromSeed(bz)
	pub := priv.Public().(ed25519.PublicKey)
	privKeyCbor, err := cbor.Encode(priv.Seed())
	require.NoError(t, err)
	pubKeyCbor, err := cbor.Encode(pub)
	require.NoError(t, err)
	require.Equal(t, skey1Cbor, hex.EncodeToString(privKeyCbor))
	require.Equal(t, vkey1Cbor, hex.EncodeToString(pubKeyCbor))
}

func TestKeys_Bech32(t *testing.T) {
	hrp, data, err := bech32.Decode(skey2Bech32)
	require.NoError(t, err)
	require.Equal(t, "addr_sk", hrp)
	skey, err := bech32.ConvertBits(data, 5, 8, false)
	require.NoError(t, err)
	priv := ed25519.NewKeyFromSeed(skey)
	pub := priv.Public().(ed25519.PublicKey)
	data, err = bech32.ConvertBits(pub, 8, 5, true)
	require.NoError(t, err)
	pubBech32, err := bech32.Encode("addr_vk", data)
	require.NoError(t, err)
	require.Equal(t, vkey2Bech32, pubBech32)
	addr, err := address.NewMainnetPaymentOnly(pub)
	require.NoError(t, err)
	require.Equal(t, addrBech32, string(addr))
}
