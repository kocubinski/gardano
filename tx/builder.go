package tx

import (
	"crypto/ed25519"
	"fmt"

	"github.com/kocubinski/go-cardano/address"
	"github.com/kocubinski/go-cardano/fees"
	"github.com/kocubinski/go-cardano/protocol"
)

// TxBuilder - used to create, validate and sign transactions.
type TxBuilder struct {
	tx       *Tx
	privs    []ed25519.PrivateKey
	protocol *protocol.Protocol
}

// Sign adds a private key to create signature for witness
func (tb *TxBuilder) Sign(priv ed25519.PrivateKey) {
	tb.privs = append(tb.privs, priv)
}

// Build creates hash of transaction, signs the hash using supplied witnesses and adds them to the transaction.
func (tb *TxBuilder) Build() (tx Tx, err error) {
	hash, err := tb.tx.Hash()
	if err != nil {
		return tx, err
	}

	txKeys := []*VKeyWitness{}
	for _, prv := range tb.privs {
		publicKey := prv.Public().(ed25519.PublicKey)
		signature, err := prv.Sign(nil, hash[:], &ed25519.Options{})
		if err != nil {
			return tx, err
		}

		txKeys = append(txKeys, NewVKeyWitness(publicKey, signature[:]))
	}

	tb.tx.WitnessSet = NewTXWitness(
		txKeys...,
	)

	return *tb.tx, nil
}

// Tx returns a pointer to the transaction
func (tb *TxBuilder) Tx() (tx *Tx) {
	return tb.tx
}

// AddChangeIfNeeded calculates the excess change from UTXO inputs - outputs and adds it to the transaction body.
func (tb *TxBuilder) AddChangeIfNeeded(addr address.Address) error {
	// change is amount in utxo minus outputs minus fee
	minFee, err := tb.MinFee()
	if err != nil {
		return err
	}
	tb.tx.SetFee(minFee)
	totalI, totalO := tb.getTotalInputOutputs()

	change := totalI - totalO - uint(tb.tx.Body.Fee)
	if change > 0 {
		tb.tx.AddOutputs(
			NewTxOutput(
				addr,
				change,
			),
		)
	}
	return nil
}

// SetTTL sets the time to live for the transaction.
func (tb *TxBuilder) SetTTL(ttl uint32) {
	tb.tx.Body.TTL = ttl
}

// SetMemo sets the memo for the transaction as specified in https://cips.cardano.org/cip/CIP-20
func (tb *TxBuilder) SetMemo(memos ...string) error {
	if len(memos) == 0 {
		return nil
	}
	for _, m := range memos {
		if len(m) > 64 {
			return fmt.Errorf("memo is too long: %d", len(m))
		}
	}

	if tb.tx.Metadata == nil {
		tb.tx.Metadata = make(map[uint64]any)
	}
	msgEnvelope := make(map[string][]string)
	msgEnvelope["msg"] = memos

	if _, ok := tb.tx.Metadata[674]; ok {
		return fmt.Errorf("memo already set")
	}
	tb.tx.Metadata[674] = msgEnvelope

	return nil
}

func (tb TxBuilder) getTotalInputOutputs() (inputs, outputs uint) {
	for _, inp := range tb.tx.Body.Inputs.TxIns {
		inputs += inp.Amount
	}
	for _, out := range tb.tx.Body.Outputs {
		outputs += uint(out.Amount)
	}

	return
}

// MinFee calculates the minimum fee for the provided transaction.
func (tb TxBuilder) MinFee() (fee uint, err error) {
	feeTx := Tx{
		Body: TxBody{
			Inputs:  tb.tx.Body.Inputs,
			Outputs: tb.tx.Body.Outputs,
			Fee:     tb.tx.Body.Fee,
			TTL:     tb.tx.Body.TTL,
		},
		WitnessSet: tb.tx.WitnessSet,
		Valid:      true,
		Metadata:   tb.tx.Metadata,
	}
	err = feeTx.CalculateAuxiliaryDataHash()
	if err != nil {
		return
	}
	if feeTx.WitnessSet.VKeys == nil {
		feeTx.WitnessSet.VKeys = &VKeyWitnessSet{}
		vWitness := NewVKeyWitness(
			make([]byte, 32),
			make([]byte, 64),
		)
		feeTx.WitnessSet.VKeys.Append(vWitness)
	}

	totalI, totalO := tb.getTotalInputOutputs()

	if totalI != (totalO) {
		inner_addr := address.Address("addr_test1qqe6zztejhz5hq0xghlf72resflc4t2gmu9xjlf73x8dpf88d78zlt4rng3ccw8g5vvnkyrvt96mug06l5eskxh8rcjq2wyd63")
		feeTx.Body.Outputs = append(feeTx.Body.Outputs, NewTxOutput(inner_addr, (totalI-totalO-200000)))
	}
	lfee := fees.NewLinearFee(tb.protocol.TxFeePerByte, tb.protocol.TxFeeFixed)
	// The fee may have increased enough to increase the number of bytes, so do one more pass
	fee, _ = feeTx.Fee(lfee)
	feeTx.Body.Fee = uint64(fee)
	fee, _ = feeTx.Fee(lfee)

	return
}

// AddInputs adds inputs to the transaction body
func (tb *TxBuilder) AddInputs(inputs ...*TxInput) {
	tb.tx.AddInputs(inputs...)
}

// AddOutputs add outputs to the transaction body
func (tb *TxBuilder) AddOutputs(outputs ...TxOutput) {
	tb.tx.AddOutputs(outputs...)
}

// NewTxBuilder returns pointer to a new TxBuilder.
func NewTxBuilder(pr *protocol.Protocol, privs []ed25519.PrivateKey) *TxBuilder {
	return &TxBuilder{
		tx:       NewTx(),
		privs:    privs,
		protocol: pr,
	}
}
