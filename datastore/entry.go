package datastore

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
)

type entry struct {
	key   string
	value string
	sum   []byte
}

func (e *entry) getLength() int64 {
	return int64(len(e.key) + len(e.value) + 12)
}

func (e *entry) Encode() []byte {
	kl := len(e.key)
	vl := len(e.value)
	size := kl + vl + 32
	res := make([]byte, size)
	binary.LittleEndian.PutUint32(res, uint32(size))
	binary.LittleEndian.PutUint32(res[4:], uint32(kl))
	binary.LittleEndian.PutUint32(res[8:], uint32(vl))
	copy(res[12:], e.key)
	copy(res[kl+12:], e.value)
	data := make([]byte, size-20)
	copy(data, res[:size-19])
	sum := sha1.Sum(data)
	copy(res[size-20:], sum[:])

	return res
}

func (e *entry) Decode(input []byte) {
	kl := binary.LittleEndian.Uint32(input[4:])
	keyBuffer := make([]byte, kl)
	copy(keyBuffer, input[12:kl+12])
	e.key = string(keyBuffer)

	vl := binary.LittleEndian.Uint32(input[8:])
	valueBuffer := make([]byte, vl)
	copy(valueBuffer, input[kl+12:kl+12+vl])
	e.value = string(valueBuffer)
	e.sum = make([]byte, 20)
	copy(e.sum, input[kl+vl+12:])
}

func readValue(in *bufio.Reader) (string, error) {
	n := 12
	header, err := in.Peek(n)
	if err != nil {
		return "", err
	}
	keySize := int(binary.LittleEndian.Uint32(header[4:]))
	valueSize := int(binary.LittleEndian.Uint32(header[8:]))

	data, err := in.Peek(n + keySize + valueSize)
	if err != nil {
		return "", err
	}

	_, err = in.Discard(n + keySize)
	if err != nil {
		return "", err
	}

	valueData, err := in.Peek(valueSize)
	if err != nil {
		return "", err
	}
	if len(valueData) != valueSize {
		return "", fmt.Errorf("cannot read value")
	}

	_, err = in.Discard(valueSize)
	if err != nil {
		return "", err
	}

	sum, err := in.Peek(n + 8)
	if err != nil {
		return "", err
	}
	realSum := sha1.Sum(data)
	if !bytes.Equal(sum, realSum[:]) {
		return "", errors.New("sha1 sum mismatch")
	}

	return string(valueData), nil
}
