package datastore

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	outFileName = "current-data"
	bufSize     = 8192
)

var ErrNotFound = fmt.Errorf("record does not exist")

type hashIndex map[string]int64

type Db struct {
	out                *os.File
	outPath            string
	outOffset          int64
	dir                string
	segmentSize        int64
	lastSegmentIndex   int
	keyPositions       chan *KeyPosition
	indexOperations    chan IndexOperation
	putOperation       chan entry
	putOperationFinish chan error
	index              hashIndex
	segments           []*Segment
}

type Segment struct {
	outOffset int64
	index     hashIndex
	filePath  string
}

type IndexOperation struct {
	Write bool
	key   string
	index int64
}

type KeyPosition struct {
	segment  *Segment
	position int64
}

func NewDb(dir string, segmentSize int64) (*Db, error) {
	db := &Db{
		dir:                dir,
		segmentSize:        segmentSize,
		segments:           make([]*Segment, 0),
		indexOperations:    make(chan IndexOperation),
		keyPositions:       make(chan *KeyPosition),
		putOperation:       make(chan entry),
		putOperationFinish: make(chan error),
	}

	err := db.createSegment()
	if err != nil {
		return nil, err
	}

	err = db.recover()
	if err != nil && err != io.EOF {
		return nil, err
	}
	db.startIndexRoutine()
	db.PutRoutine()
	return db, nil
}

func (db *Db) startIndexRoutine() {
	go func() {
		for op := range db.indexOperations {
			if !op.Write {
				segment, position, err := db.getSegmentAndPosition(op.key)
				if err != nil {
					db.keyPositions <- nil
					continue
				}
				db.keyPositions <- &KeyPosition{
					segment:  segment,
					position: position,
				}
				continue
			}
			db.setKey(op.key, op.index)
		}
	}()
}

func (db *Db) PutRoutine() {
	go func() {
		for entry := range db.putOperation {
			length := entry.getLength()

			stat, err := db.out.Stat()
			if err != nil {
				db.putOperationFinish <- err
				continue
			}

			if stat.Size()+length > db.segmentSize {
				if err := db.createSegment(); err != nil {
					db.putOperationFinish <- err
					continue
				}
			}

			n, err := db.out.Write(entry.Encode())
			if err != nil {
				db.putOperationFinish <- err
				continue
			}

			db.indexOperations <- IndexOperation{
				Write: true,
				key:   entry.key,
				index: int64(n),
			}
			db.putOperationFinish <- nil
		}
	}()
}

func (db *Db) createSegment() error {
	filePath := db.getNewFileName()
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		return err
	}

	newSegment := &Segment{
		filePath: filePath,
		index:    make(hashIndex),
	}

	db.out = f
	db.outOffset = 0
	db.segments = append(db.segments, newSegment)

	if len(db.segments) >= 3 {
		go db.compactOldSegments()
	}

	return nil
}

func (db *Db) getPos(key string) *KeyPosition {
	readOp := IndexOperation{
		Write: false,
		key:   key,
	}
	db.indexOperations <- readOp
	return <-db.keyPositions
}

func (db *Db) getNewFileName() string {
	result := filepath.Join(db.dir, fmt.Sprintf("%s%d", outFileName, db.lastSegmentIndex))
	db.lastSegmentIndex++
	return result
}

func (db *Db) compactOldSegments() {
	filePath := db.getNewFileName()
	newSegment := &Segment{
		filePath: filePath,
		index:    make(hashIndex),
	}
	var offset int64

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return
	}
	defer f.Close()

	lastSegmentIndex := len(db.segments) - 2
	for i := 0; i <= lastSegmentIndex; i++ {
		s := db.segments[i]
		for key, index := range s.index {
			if i < lastSegmentIndex && checkKeyInSegments(db.segments[i+1:lastSegmentIndex+1], key) {
				continue
			}

			value, _ := s.getFromSegment(index)
			e := entry{
				key:   key,
				value: value,
			}

			n, err := f.Write(e.Encode())
			if err == nil {
				newSegment.index[key] = offset
				offset += int64(n)
			}
		}
	}

	db.segments = []*Segment{newSegment, db.getLastSegment()}
}

func checkKeyInSegments(segments []*Segment, key string) bool {
	for _, s := range segments {
		if _, ok := s.index[key]; ok {
			return true
		}
	}
	return false
}

func (db *Db) recover() error {
	var err error
	var buf [bufSize]byte
	in := bufio.NewReaderSize(db.out, bufSize)
	for err == nil {
		header, err := in.Peek(bufSize)
		if err == io.EOF {
			if len(header) == 0 {
				return err
			}
		} else if err != nil {
			return err
		}
		size := binary.LittleEndian.Uint32(header)
		var data []byte

		if size < bufSize {
			data = buf[:size]
		} else {
			data = make([]byte, size)
		}

		n, err := in.Read(data)
		if err == nil {
			if n != int(size) {
				return fmt.Errorf("corrupted file")
			}

			var e entry
			e.Decode(data)
			db.setKey(e.key, int64(n))
		}
	}
	return err
}

func (db *Db) Close() error {
	return db.out.Close()
}

func (db *Db) setKey(key string, n int64) {
	db.getLastSegment().index[key] = db.outOffset
	db.outOffset += n
}

func (db *Db) getSegmentAndPosition(key string) (*Segment, int64, error) {
	for i := range db.segments {
		s := db.segments[len(db.segments)-i-1]
		pos, ok := s.index[key]
		if ok {
			return s, pos, nil
		}
	}

	return nil, 0, ErrNotFound
}

func (db *Db) Get(key string) (string, error) {
	keyPlacement := db.getPos(key)
	if keyPlacement != nil {
		return "", ErrNotFound
	}
	value, err := keyPlacement.segment.getFromSegment(keyPlacement.position)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (db *Db) getLastSegment() *Segment {
	return db.segments[len(db.segments)-1]
}

func (db *Db) Put(key, value string) error {
	entry := entry{
		key:   key,
		value: value,
	}
	db.putOperation <- entry
	return <-db.putOperationFinish
}

func (s *Segment) getFromSegment(position int64) (string, error) {
	file, err := os.Open(s.filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = file.Seek(position, 0)
	if err != nil {
		return "", err
	}

	reader := bufio.NewReader(file)
	value, err := readValue(reader)
	if err != nil {
		return "", err
	}
	return value, nil
}
