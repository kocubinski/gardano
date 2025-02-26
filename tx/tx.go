package tx

import (
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"
	"github.com/kocubinski/go-cardano/address"
	"github.com/kocubinski/go-cardano/fees"
	"golang.org/x/crypto/blake2b"
)

var (
	txDecMode cbor.DecMode
	txEncMode cbor.EncMode
)

func init() {
	// Use signedCWT struct defined in "Decoding CWT" example.

	// Create TagSet (safe for concurrency).
	tags := cbor.NewTagSet()
	err := tags.Add(
		cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
		reflect.TypeOf(TxInputSet{}),
		258)
	if err != nil {
		panic(err)
	}

	// Create DecMode with immutable tags.
	txDecMode, _ = cbor.DecOptions{}.DecModeWithTags(tags)

	// Create EncMode with immutable tags.
	txEncMode, _ = cbor.EncOptions{}.EncModeWithTags(tags)
}

type Tx struct {
	_        struct{} `cbor:",toarray"`
	Body     *TxBody
	Witness  *Witness
	Valid    bool
	Metadata interface{}
}

// NewTx returns a pointer to a new Transaction
func NewTx() *Tx {
	return &Tx{
		Body:    NewTxBody(),
		Witness: NewTXWitness(),
		Valid:   true,
	}
}

// Bytes returns a slice of cbor marshalled bytes
func (t *Tx) Bytes() ([]byte, error) {
	if err := t.CalculateAuxiliaryDataHash(); err != nil {
		return nil, err
	}
	bytes, err := txEncMode.Marshal(t)
	return bytes, err
}

// Hex returns hex encoding of the transacion bytes
func (t *Tx) Hex() (string, error) {
	bytes, err := t.Bytes()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Hash performs a blake2b hash of the transaction body and returns a slice of [32]byte
func (t *Tx) Hash() ([32]byte, error) {
	if err := t.CalculateAuxiliaryDataHash(); err != nil {
		return [32]byte{}, err
	}
	txBody, err := cbor.Marshal(t.Body)
	if err != nil {
		var bt [32]byte
		return bt, err
	}

	txHash := blake2b.Sum256(txBody)
	return txHash, nil
}

// Fee returns the fee(in lovelaces) required by the transaction from the linear formula
// fee = txFeeFixed + txFeePerByte*tx_len_in_bytes
func (t *Tx) Fee(lfee *fees.LinearFee) (uint, error) {
	if err := t.CalculateAuxiliaryDataHash(); err != nil {
		return 0, err
	}
	txCbor, err := cbor.Marshal(t)
	if err != nil {
		return 0, err
	}
	txBodyLen := len(txCbor)
	fee := lfee.TxFeeFixed + lfee.TxFeePerByte*uint(txBodyLen)

	return fee, nil
}

// SetFee sets the fee
func (t *Tx) SetFee(fee uint) {
	t.Body.Fee = uint64(fee)
}

func (t *Tx) CalculateAuxiliaryDataHash() error {
	if t.Metadata != nil {
		mdBytes, err := cbor.Marshal(&t.Metadata)
		if err != nil {
			return fmt.Errorf("cannot serialize metadata: %w", err)
		}
		auxHash := blake2b.Sum256(mdBytes)
		t.Body.AuxiliaryDataHash = auxHash[:]
	}
	return nil
}

// AddInputs adds the inputs to the transaction body
func (t *Tx) AddInputs(inputs ...*TxInput) error {
	t.Body.Inputs = append(t.Body.Inputs, inputs...)

	return nil
}

// AddOutputs adds the outputs to the transaction body
func (t *Tx) AddOutputs(outputs ...*TxOutput) error {
	t.Body.Outputs = append(t.Body.Outputs, outputs...)

	return nil
}

type TxInputSet []*TxInput

// TxBody contains the inputs, outputs, fee and titme to live for the transaction.
type TxBody struct {
	Inputs            TxInputSet  `cbor:"0,keyasint"`
	Outputs           []*TxOutput `cbor:"1,keyasint"`
	Fee               uint64      `cbor:"2,keyasint"`
	TTL               uint32      `cbor:"3,keyasint,omitempty"`
	AuxiliaryDataHash []byte      `cbor:"7,keyasint,omitempty"`
}

// NewTxBody returns a pointer to a new transaction body.
func NewTxBody() *TxBody {
	return &TxBody{
		Inputs:  make([]*TxInput, 0),
		Outputs: make([]*TxOutput, 0),
	}
}

// Bytes returns a slice of cbor Marshalled bytes.
func (b *TxBody) Bytes() ([]byte, error) {
	bytes, err := txEncMode.Marshal(b)
	return bytes, err
}

// Hex returns hex encoded string of the transaction bytes.
func (b *TxBody) Hex() (string, error) {
	by, err := b.Bytes()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(by), nil
}

type TxInput struct {
	cbor.Marshaler

	TxHash []byte
	Index  uint16
	Amount uint
}

// NewTxInput creates and returns a *TxInput from Transaction Hash(Hex Encoded), Transaction Index and Amount.
func NewTxInput(txHash string, txIx uint16, amount uint) *TxInput {
	hash, _ := hex.DecodeString(txHash)

	return &TxInput{
		TxHash: hash,
		Index:  txIx,
		Amount: amount,
	}
}

func (txI *TxInput) MarshalCBOR() ([]byte, error) {
	type arrayInput struct {
		_      struct{} `cbor:",toarray"`
		TxHash []byte
		Index  uint16
	}
	input := arrayInput{
		TxHash: txI.TxHash,
		Index:  txI.Index,
	}
	return cbor.Marshal(input)
}

type TxOutput struct {
	_       struct{} `cbor:",toarray"`
	Address address.Address
	Amount  uint
}

func NewTxOutput(addr address.Address, amount uint) *TxOutput {
	return &TxOutput{
		Address: addr,
		Amount:  amount,
	}
}

type Witness struct {
	Keys []*VKeyWitness `cbor:"0,keyasint,omitempty"`
}

// NewTXWitness returns a pointer to a Witness created from VKeyWitnesses.
func NewTXWitness(keys ...*VKeyWitness) *Witness {
	if len(keys) == 0 {
		return &Witness{
			Keys: make([]*VKeyWitness, 0),
		}
	}

	return &Witness{
		Keys: keys,
	}
}

// VKeyWitness - Witness for use with Shelley based transactions
type VKeyWitness struct {
	_         struct{} `cbor:",toarray"`
	VKey      []byte
	Signature []byte
}

// NewVKeyWitness creates a Witness for Shelley Based transactions from a verification key and transaction signature.
func NewVKeyWitness(vkey, signature []byte) *VKeyWitness {
	return &VKeyWitness{
		VKey: vkey, Signature: signature,
	}
}

// BootstrapWitness for use with Byron/Legacy based transactions
type BootstrapWitness struct {
	_          struct{} `cbor:",toarray"`
	VKey       []byte
	Signature  []byte
	ChainCode  []byte
	Attributes []byte
}
