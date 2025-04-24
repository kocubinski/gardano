package tx

import (
	"crypto/ed25519"
	"fmt"

	"github.com/kocubinski/gardano/address"
	utxocardano "github.com/utxorpc/go-codegen/utxorpc/v1alpha/cardano"
)

// TxBuilder - used to create, validate and sign transactions.
type TxBuilder struct {
	tx           *Tx
	witnessCount int
	protocol     *utxocardano.PParams
	changeAddr   address.Address
}

func (tb *TxBuilder) CalculateFee() error {
	if _, err := tb.tx.Hash(); err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	// empty witness set for fee calc
	tb.tx.WitnessSet.VKeys = &VKeyWitnessSet{}
	for range tb.witnessCount {
		tb.tx.WitnessSet.VKeys.Append(NewVKeyWitness(make([]byte, 32), make([]byte, 64)))
	}

	// first pass to estimate size without fee
	txCbor, err := tb.tx.Bytes()
	if err != nil {
		return err
	}
	tb.tx.Body.Fee = tb.protocol.MinFeeCoefficient*uint64(len(txCbor)) + tb.protocol.MinFeeConstant
	// second pass with fee
	txCbor, err = tb.tx.Bytes()
	if err != nil {
		return err
	}
	tb.tx.Body.Fee = tb.protocol.MinFeeCoefficient*uint64(len(txCbor)) + tb.protocol.MinFeeConstant
	// subtract the fee from the outputs if one is a change address; hope this doesn't change size
	for i, txOut := range tb.tx.Body.Outputs {
		if txOut.Address.Equals(tb.changeAddr) {
			txOut.Amount -= tb.tx.Body.Fee
			tb.tx.Body.Outputs[i] = txOut
			break
		}
	}

	return nil
}

// Returns a transaction signed by the provided private keys.
func (tb *TxBuilder) Sign(privateKeys []ed25519.PrivateKey) (tx Tx, err error) {
	tx = *tb.tx

	hash, err := tx.Hash()
	if err != nil {
		return tx, err
	}

	// sign the transaction with the private keys
	txKeys := []*VKeyWitness{}
	for _, prv := range privateKeys {
		publicKey := prv.Public().(ed25519.PublicKey)
		signature, err := prv.Sign(nil, hash[:], &ed25519.Options{})
		if err != nil {
			return tx, err
		}

		txKeys = append(txKeys, NewVKeyWitness(publicKey, signature))
	}

	tx.WitnessSet = NewTXWitness(
		txKeys...,
	)

	return tx, nil
}

// Tx returns a pointer to the transaction
func (tb *TxBuilder) Tx() (tx *Tx) {
	return tb.tx
}

// AddChangeIfNeeded calculates the excess change from UTXO inputs - outputs and adds it to the transaction body.
func (tb *TxBuilder) AddChangeIfNeeded(addr address.Address) error {
	tb.changeAddr = addr
	totalI, totalO := tb.getTotalInputOutputs()
	change := totalI - totalO
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
func (tb *TxBuilder) SetMemo(memo string) error {
	if len(memo) == 0 {
		return nil
	}
	memos := []string{}
	for len(memo) > 64 {
		memos = append(memos, memo[:64])
		memo = memo[64:]
	}
	if len(memo) > 0 {
		memos = append(memos, memo)
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

func (tb TxBuilder) getTotalInputOutputs() (inputs, outputs uint64) {
	for _, inp := range tb.tx.Body.Inputs.TxIns {
		inputs += inp.Amount
	}
	for _, out := range tb.tx.Body.Outputs {
		outputs += out.Amount
	}

	return
}

// AddInputs adds inputs to the transaction body
func (tb *TxBuilder) AddInputs(inputs ...TxInput) {
	tb.tx.AddInputs(inputs...)
}

// AddOutputs add outputs to the transaction body
func (tb *TxBuilder) AddOutputs(outputs ...TxOutput) {
	tb.tx.AddOutputs(outputs...)
}

// NewTxBuilder returns pointer to a new TxBuilder.
func NewTxBuilder(pr *utxocardano.PParams, opts ...TxBuilderOption) *TxBuilder {
	builder := &TxBuilder{
		tx:           NewTx(),
		witnessCount: 1,
		protocol:     pr,
	}
	for _, opt := range opts {
		opt(builder)
	}
	return builder
}

type TxBuilderOption func(*TxBuilder)

func WithWitnessCount(count int) TxBuilderOption {
	return func(tb *TxBuilder) {
		tb.witnessCount = count
	}
}
