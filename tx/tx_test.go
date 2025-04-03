package tx_test

import (
	"encoding/hex"
	"testing"

	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/kocubinski/gardano/address"
	. "github.com/kocubinski/gardano/tx"
	"github.com/stretchr/testify/require"
)

func addrFromBech32(t *testing.T, addrBech32 string) address.Address {
	addr, err := address.NewAddressFromBech32(addrBech32)
	require.NoError(t, err)
	return addr
}

func Test_TransactionSpec(t *testing.T) {
	cases := []struct {
		name string
		tx   Tx
		cbor string
	}{
		{
			name: "one input, no outputs",
			tx: Tx{
				Body: TxBody{
					Inputs: TxInputSet{
						TxIns: []TxInput{
							NewTxInput("086838187822234a2153763a74daea139f29cf8753cb84f6e0c904e1db0ea3ab", 0, 2832783),
						},
					},
					Outputs: make([]TxOutput, 0),
				},
				Valid: true,
			},
			cbor: "84a300d9010281825820086838187822234a2153763a74daea139f29cf8753cb84f6e0c904e1db0ea3ab0001800200a0f5f6",
		},
		{
			name: "one input, one output, fee",
			tx: Tx{
				Body: TxBody{
					Inputs: TxInputSet{
						TxIns: []TxInput{
							NewTxInput("086838187822234a2153763a74daea139f29cf8753cb84f6e0c904e1db0ea3ab", 0, 2832783),
						},
					},
					Outputs: []TxOutput{
						NewTxOutput(addrFromBech32(t, "addr1v9f785wjgm4w0ky6lrjp4ecfj7dunzhql83ratqlpenqn2ssnlkjz"), 2500000),
					},
					Fee: 166249,
				},
				Valid: true,
			},
			cbor: "84a300d9010281825820086838187822234a2153763a74daea139f29cf8753cb84f6e0c904e1db0ea3ab00018182581d6153e3d1d246eae7d89af8e41ae709979bc98ae0f9e23eac1f0e6609aa1a002625a0021a00028969a0f5f6",
		},
		{
			name: "one input, two outputs, fee",
			tx: Tx{
				Body: TxBody{
					Inputs: TxInputSet{
						TxIns: []TxInput{
							NewTxInput("086838187822234a2153763a74daea139f29cf8753cb84f6e0c904e1db0ea3ab", 0, 2832783),
						},
					},
					Outputs: []TxOutput{
						NewTxOutput(addrFromBech32(t, "addr1v9f785wjgm4w0ky6lrjp4ecfj7dunzhql83ratqlpenqn2ssnlkjz"), 2500000),
						NewTxOutput(addrFromBech32(t, "addr1v8hc0xl88ehea8698tjejhwjum87hsusdpne787znge7sps4x4v8v"), 166534),
					},
					Fee: 166249,
				},
				Valid: true,
			},
			cbor: "84a300d9010281825820086838187822234a2153763a74daea139f29cf8753cb84f6e0c904e1db0ea3ab00018282581d6153e3d1d246eae7d89af8e41ae709979bc98ae0f9e23eac1f0e6609aa1a002625a082581d61ef879be73e6f9e9f453ae5995dd2e6cfebc39068679f1fc29a33e8061a00028a86021a00028969a0f5f6",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bz, err := c.tx.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			hexCbor := hex.EncodeToString(bz)
			if hexCbor != c.cbor {
				t.Fatalf("expected %s, got %s", c.cbor, hexCbor)
			}
		})
	}
}

func Test_Metadata(t *testing.T) {
	cborMetadata := "a11902a2a1636d7367816b666f6f2b6261722d62617a"
	cborBytes, err := hex.DecodeString(cborMetadata)
	require.NoError(t, err)
	lazyValue := &cbor.LazyValue{}
	err = lazyValue.UnmarshalCBOR(cborBytes)
	require.NoError(t, err)
	require.Nil(t, lazyValue.Value())
	res, err := lazyValue.Decode()
	require.NoError(t, err)
	require.Equal(t, res, lazyValue.Value())

	_, err = lazyValue.MarshalJSON()
	require.NoError(t, err)

	memo, err := DecodeMemoFromMetadata(lazyValue)
	require.NoError(t, err)
	require.Equal(t, "foo+bar-baz", memo)
}
