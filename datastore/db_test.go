package datastore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestDb_Put(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := NewDb(dir, 150)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	pairs := [][]string{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
	}

	t.Run("put/get", func(t *testing.T) {
		for _, pair := range pairs {
			pair := pair
			t.Run(pair[0], func(t *testing.T) {
				t.Parallel()
				err := db.Put(pair[0], pair[1])
				if err != nil {
					t.Errorf("Cannot put %s: %s", pair[0], err)
				}
				value, err := db.Get(pair[0])
				if err != nil {
					t.Errorf("Cannot get %s: %s", pair[0], err)
				}
				if value != pair[1] {
					t.Errorf("Bad value returned expected %s, got %s", pair[1], value)
				}
			})
		}
	})

	t.Run("file growth", func(t *testing.T) {
		outFile, err := os.Open(filepath.Join(dir, outFileName+"0"))
		if err != nil {
			t.Fatal(err)
		}
		defer outFile.Close()

		for _, pair := range pairs {
			pair := pair // Create a local copy of the loop variable
			err := db.Put(pair[0], pair[1])
			if err != nil {
				t.Errorf("Cannot put %s: %s", pair[0], err)
			}
		}

		outInfo, err := outFile.Stat()
		if err != nil {
			t.Fatal(err)
		}
		size1 := outInfo.Size() / 2
		if size1*2 != outInfo.Size() {
			t.Errorf("Unexpected size (%d vs %d)", size1, outInfo.Size())
		}
	})

	t.Run("new db process", func(t *testing.T) {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}

		db, err = NewDb(dir, 100)
		if err != nil {
			t.Fatal(err)
		}

		for _, pair := range pairs {
			pair := pair // Create a local copy of the loop variable
			t.Run(pair[0], func(t *testing.T) {
				t.Parallel()
				value, err := db.Get(pair[0])
				if err != nil {
					t.Errorf("Cannot put %s: %s", pair[0], err)
				}
				if value != pair[1] {
					t.Errorf("Bad value returned expected %s, got %s", pair[1], value)
				}
			})
		}
	})
}

func TestDb_Segmentation(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := NewDb(dir, 85)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	t.Run("should create new file", func(t *testing.T) {
		db.Put("key1", "value1")
		db.Put("key2", "value2")
		db.Put("key3", "value3")
		db.Put("key2", "value5")

		assertSegmentsCount(t, db, 2)
	})

	t.Run("should start segmentation", func(t *testing.T) {
		db.Put("key4", "value4")

		assertSegmentsCount(t, db, 3)

		time.Sleep(2 * time.Second)

		assertSegmentsCount(t, db, 2)
	})

	t.Run("shouldn't store duplicates", func(t *testing.T) {
		file, err := os.Open(db.segments[0].filePath)
		defer file.Close()

		if err != nil {
			t.Error(err)
		}
		inf, _ := file.Stat()
		assertFileSize(t, inf, 126)
	})

	t.Run("shouldn't store new values of duplicate keys", func(t *testing.T) {
		value, _ := db.Get("key2")
		assertEqual(t, value, "value5")
	})
}

func TestDb_Checksum(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := NewDb(dir, 85)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Put("key1", "value1")

	t.Run("get value", func(t *testing.T) {
		_, err := db.Get("key1")
		if err != nil {
			t.Errorf("something went wrong during getting value: %v", err)
		}
	})

	file, err := os.OpenFile(db.outPath, os.O_RDWR, 0o655)
	if err != nil {
		t.Fatal(err)
	}

	_, err = file.WriteAt([]byte{0x59}, int64(3))
	if err != nil {
		file.Close()
		t.Fatal(err)
	}
	file.Close()

	t.Run("wrong during get value", func(t *testing.T) {
		_, err := db.Get("key1")
		if err == nil || !regexp.MustCompile("sha1").MatchString(err.Error()) {
			t.Errorf("Expected error containing 'sha', but got: %v", err)
		}
	})
}

func assertSegmentsCount(t *testing.T, db *Db, expectedCount int) {
	t.Helper()
	if len(db.segments) != expectedCount {
		t.Errorf("Something went wrong with segmentation. Expected %d files, got %d", expectedCount, len(db.segments))
	}
}

func assertFileSize(t *testing.T, fileInfo os.FileInfo, expectedSize int64) {
	t.Helper()
	if fileInfo.Size() != expectedSize {
		t.Errorf("Something went wrong with segmentation. Expected size %d, got %d", expectedSize, fileInfo.Size())
	}
}

func assertEqual(t *testing.T, got, expected interface{}) {
	t.Helper()
	if got != expected {
		t.Errorf("Expected: %v, Got: %v", expected, got)
	}
}
