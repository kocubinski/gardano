package tx

import (
	"bytes"
	"encoding/binary"
	"math"
)

const (
	maxAdditionalInformationWithoutArgument       = 23
	additionalInformationWith1ByteArgument        = 24
	additionalInformationWith2ByteArgument        = 25
	additionalInformationWith4ByteArgument        = 26
	additionalInformationWith8ByteArgument        = 27
	cborTypeTag                             uint8 = 0xc0
)

// encodeHead writes CBOR head of specified type t and returns number of bytes written.
func encodeHead(e *bytes.Buffer, t byte, n uint64) int {
	if n <= maxAdditionalInformationWithoutArgument {
		const headSize = 1
		e.WriteByte(t | byte(n))
		return headSize
	}

	if n <= math.MaxUint8 {
		const headSize = 2
		scratch := [headSize]byte{
			t | byte(additionalInformationWith1ByteArgument),
			byte(n),
		}
		e.Write(scratch[:])
		return headSize
	}

	if n <= math.MaxUint16 {
		const headSize = 3
		var scratch [headSize]byte
		scratch[0] = t | byte(additionalInformationWith2ByteArgument)
		binary.BigEndian.PutUint16(scratch[1:], uint16(n))
		e.Write(scratch[:])
		return headSize
	}

	if n <= math.MaxUint32 {
		const headSize = 5
		var scratch [headSize]byte
		scratch[0] = t | byte(additionalInformationWith4ByteArgument)
		binary.BigEndian.PutUint32(scratch[1:], uint32(n))
		e.Write(scratch[:])
		return headSize
	}

	const headSize = 9
	var scratch [headSize]byte
	scratch[0] = t | byte(additionalInformationWith8ByteArgument)
	binary.BigEndian.PutUint64(scratch[1:], n)
	e.Write(scratch[:])
	return headSize
}
