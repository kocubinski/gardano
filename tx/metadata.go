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
	md, ok := val.Value().(map[any]any)
	if !ok {
		return "", fmt.Errorf("failed to cast metadata to map[any]any")
	}
	envelope, ok := md[uint64(674)].(map[any]any)
	if !ok {
		return "", fmt.Errorf("failed to cast metadata envelope to map[any]any")
	}
	anyMsgs, ok := envelope["msg"].([]any)
	if !ok {
		return "", fmt.Errorf("failed to cast metadata envelope msg to []string")
	}
	var memo string
	for _, anyMsg := range anyMsgs {
		msg, ok := anyMsg.(string)
		if !ok {
			return "", fmt.Errorf("failed to cast metadata envelope msg to string")
		}
		memo += msg
	}
	return memo, nil
}
