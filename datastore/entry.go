package datastore

import (
	"bufio"
	"encoding/binary"
	"fmt"
)

type entry struct {
	key   string
	value string
}

func (e *entry) getLength() int64 {
	return int64(len(e.key) + len(e.value) + 12)
}

func (e *entry) Encode() []byte {
	kl := len(e.key)
	vl := len(e.value)
	size := kl + vl + 12
	res := make([]byte, size)
	binary.LittleEndian.PutUint32(res[:4], uint32(size))
	binary.LittleEndian.PutUint32(res[4:8], uint32(kl))
	copy(res[8:8+kl], e.key)
	binary.LittleEndian.PutUint32(res[8+kl:12+kl], uint32(vl))
	copy(res[12+kl:], e.value)
	return res
}

func (e *entry) Decode(input []byte) {
	kl := binary.LittleEndian.Uint32(input[4:8])
	e.key = string(input[8 : 8+kl])
	vl := binary.LittleEndian.Uint32(input[8+kl : 12+kl])
	e.value = string(input[12+kl : 12+kl+vl])
}

func readValue(in *bufio.Reader) (string, error) {
	header, err := in.Peek(8)
	if err != nil {
		return "", err
	}
	keySize := int(binary.LittleEndian.Uint32(header[4:8]))
	_, err = in.Discard(keySize + 8)
	if err != nil {
		return "", err
	}

	header, err = in.Peek(4)
	if err != nil {
		return "", err
	}
	valSize := int(binary.LittleEndian.Uint32(header))
	_, err = in.Discard(4)
	if err != nil {
		return "", err
	}

	data := make([]byte, valSize)
	n, err := in.Read(data)
	if err != nil {
		return "", err
	}
	if n != valSize {
		return "", fmt.Errorf("can't read value bytes (read %d, expected %d)", n, valSize)
	}

	return string(data), nil
}
