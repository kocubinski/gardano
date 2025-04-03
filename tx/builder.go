package tx

import (
	"crypto/ed25519"
	"fmt"

	"github.com/kocubinski/gardano/address"
	utxocardano "github.com/utxorpc/go-codegen/utxorpc/v1alpha/cardano"
)

// TxBuilder - used to create, validate and sign transactions.
type TxBuilder struct {
	tx         *Tx
	privs      []ed25519.PrivateKey
	protocol   *utxocardano.PParams
	changeAddr address.Address
}

// Sign adds a private key to create signature for witness
func (tb *TxBuilder) Sign(priv ed25519.PrivateKey) {
	tb.privs = append(tb.privs, priv)
}

// Build creates hash of transaction, signs the hash using supplied witnesses and adds them to the transaction.
func (tb *TxBuilder) Build() (tx Tx, err error) {
	tx = *tb.tx
	hash, err := tx.Hash()
	if err != nil {
		return tx, err
	}

	// empty witness set to calculate the fee
	tx.WitnessSet.VKeys = &VKeyWitnessSet{}
	tx.WitnessSet.VKeys.Append(NewVKeyWitness(make([]byte, 32), make([]byte, 64)))

	txCbor, err := tx.Bytes()
	if err != nil {
		return tx, err
	}
	txLength := uint64(len(txCbor))
	fee := tb.protocol.MinFeeCoefficient*txLength + tb.protocol.MinFeeConstant + 200
	tx.Body.Fee = fee

	// subtract the fee from the outputs if one is a change address
	for i, txOut := range tx.Body.Outputs {
		if txOut.Address.Equals(tb.changeAddr) {
			txOut.Amount -= fee
			tx.Body.Outputs[i] = txOut
			break
		}
	}

	// rehash the transaction with the fee set
	hash, err = tx.Hash()
	if err != nil {
		return tx, err
	}

	// sign the transaction with the private keys
	txKeys := []*VKeyWitness{}
	for _, prv := range tb.privs {
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
	change := totalI - totalO - tb.tx.Body.Fee
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
func NewTxBuilder(pr *utxocardano.PParams, privs []ed25519.PrivateKey) *TxBuilder {
	return &TxBuilder{
		tx:       NewTx(),
		privs:    privs,
		protocol: pr,
	}
}
