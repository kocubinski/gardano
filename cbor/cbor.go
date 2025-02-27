package cbor

import (
	"bytes"
	"reflect"

	libcbor "github.com/fxamacker/cbor/v2"
)

var customTagSet libcbor.TagSet

const (
	cborTagSet = 258
	cborTagMap = 259
)

func init() {
	// Build custom tagset
	customTagSet = libcbor.NewTagSet()
	tagOpts := libcbor.TagOptions{
		EncTag: libcbor.EncTagRequired,
		DecTag: libcbor.DecTagRequired,
	}
	// Sets
	if err := customTagSet.Add(
		tagOpts,
		reflect.TypeOf(Set{}),
		cborTagSet,
	); err != nil {
		panic(err)
	}
	// Maps
	if err := customTagSet.Add(
		tagOpts,
		reflect.TypeOf(Map{}),
		cborTagMap,
	); err != nil {
		panic(err)
	}
}

type Set struct{}

type Map struct{}

func Decode(dataBytes []byte, dst any) (int, error) {
	data := bytes.NewReader(dataBytes)
	// Create a custom decoder that returns an error on unknown fields
	decOptions := libcbor.DecOptions{
		ExtraReturnErrors: libcbor.ExtraDecErrorUnknownField,
		// This defaults to 32, but there are blocks in the wild using >64 nested levels
		MaxNestedLevels: 256,
	}
	decMode, err := decOptions.DecModeWithTags(customTagSet)
	if err != nil {
		return 0, err
	}
	dec := decMode.NewDecoder(data)
	err = dec.Decode(dst)
	return dec.NumBytesRead(), err
}

func Encode(data interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	opts := libcbor.EncOptions{
		// Make sure that maps have ordered keys
		Sort: libcbor.SortCoreDeterministic,
	}
	em, err := opts.EncModeWithTags(customTagSet)
	if err != nil {
		return nil, err
	}
	enc := em.NewEncoder(buf)
	err = enc.Encode(data)
	return buf.Bytes(), err
}
