package tx

import (
	"fmt"

	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/blinklabs-io/gouroboros/ledger/common"
)

func DecodeMemo(tx common.Transaction) (string, error) {
	return DecodeMemoFromMetadata(tx.Metadata())
}

func DecodeMemoFromMetadata(val *cbor.LazyValue) (string, error) {
	if val == nil {
		return "", nil
	}
	if val.Value() == nil {
		if val.Cbor() == nil {
			return "", nil
		}
		_, err := val.Decode()
		if err != nil {
			return "", err
		}
	}

	var md map[any]any
	switch v := val.Value().(type) {
	case map[any]any:
		md = v
	case cbor.Map:
		md = map[any]any(v)
	case []any:
		// ignore this case
		return "", nil
	default:
		return "", fmt.Errorf("failed to cast metadata want: map[any]any got: %T", val.Value())
	}

	x, ok := md[uint64(674)]
	if !ok {
		return "", nil
	}
	envelope, ok := x.(map[any]any)
	if !ok {
		return "", fmt.Errorf("failed to cast metadata envelope want: map[any]any got: %T", x)
	}
	x, ok = envelope["msg"]
	if !ok {
		return "", nil
	}

	switch v := x.(type) {
	case string:
		return v, nil
	case []any:
		var memo string
		for _, anyMsg := range v {
			msg, ok := anyMsg.(string)
			if !ok {
				return "", fmt.Errorf("failed to cast memo want: string got: %T", anyMsg)
			}
			memo += msg
		}
		return memo, nil
	default:
		return "", fmt.Errorf("unknown memo type %T", x)
	}

}
