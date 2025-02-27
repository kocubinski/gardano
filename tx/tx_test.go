package tx_test

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"

	"github.com/blinklabs-io/gouroboros/ledger"
	"github.com/kocubinski/go-cardano/cbor"
	"github.com/kocubinski/go-cardano/tx"
	"github.com/stretchr/testify/require"
)

func Test_Types(t *testing.T) {
	txInputSet := tx.TxInputSet{}
	typ := reflect.TypeOf(txInputSet)
	fmt.Println(typ.AssignableTo(reflect.TypeOf(cbor.Set{})))
	fmt.Println(reflect.TypeOf(cbor.Set{}).AssignableTo(typ))
}

func Test_RandomTx(t *testing.T) {
	txCbor := "84a300d9010281825820086838187822234a2153763a74daea139f29cf8753cb84f6e0c904e1db0ea3ab0001f60200"
	txBz, err := hex.DecodeString(txCbor)
	require.NoError(t, err)
	tx, err := ledger.NewConwayTransactionBodyFromCbor(txBz)
	require.NoError(t, err)
	fmt.Println(tx)
}
