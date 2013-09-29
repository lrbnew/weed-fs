package storage

import (
	"bufio"
	"code.google.com/p/weed-fs/go/glog"
	"code.google.com/p/weed-fs/go/util"
	"fmt"
	"io"
	"os"
)

type NeedleMapper interface {
	Put(key uint64, offset uint32, size uint32) (int, error)
	Get(key uint64) (element *NeedleValue, ok bool)
	Delete(key uint64) error
	Close()
	ContentSize() uint64
	DeletedSize() uint64
	FileCount() int
	DeletedCount() int
	Visit(visit func(NeedleValue) error) (err error)
	NextFileKey(count int) uint64
}

type mapMetric struct {
	DeletionCounter     int    `json:"DeletionCounter"`
	FileCounter         int    `json:"FileCounter"`
	DeletionByteCounter uint64 `json:"DeletionByteCounter"`
	FileByteCounter     uint64 `json:"FileByteCounter"`
	MaximumFileKey      uint64 `json:"MaxFileKey"`
}

type NeedleMap struct {
	indexFile *os.File
	m         CompactMap

	//transient
	bytes []byte

	mapMetric
}

func NewNeedleMap(file *os.File) *NeedleMap {
	nm := &NeedleMap{
		m:         NewCompactMap(),
		bytes:     make([]byte, 16),
		indexFile: file,
	}
	return nm
}

const (
	RowsToRead = 1024
)

func LoadNeedleMap(file *os.File) (*NeedleMap, error) {
	nm := NewNeedleMap(file)
	e := walkIndexFile(file, func(key uint64, offset, size uint32) error {
		if key > nm.MaximumFileKey {
			nm.MaximumFileKey = key
		}
		nm.FileCounter++
		nm.FileByteCounter = nm.FileByteCounter + uint64(size)
		if offset > 0 {
			oldSize := nm.m.Set(Key(key), offset, size)
			glog.V(4).Infoln("reading key", key, "offset", offset, "size", size, "oldSize", oldSize)
			if oldSize > 0 {
				nm.DeletionCounter++
				nm.DeletionByteCounter = nm.DeletionByteCounter + uint64(oldSize)
			}
		} else {
			oldSize := nm.m.Delete(Key(key))
			glog.V(4).Infoln("removing key", key, "offset", offset, "size", size, "oldSize", oldSize)
			nm.DeletionCounter++
			nm.DeletionByteCounter = nm.DeletionByteCounter + uint64(oldSize)
		}
		return nil
	})
	glog.V(1).Infoln("max file key:", nm.MaximumFileKey)
	return nm, e
}

// walks through the index file, calls fn function with each key, offset, size
// stops with the error returned by the fn function
func walkIndexFile(r io.Reader, fn func(key uint64, offset, size uint32) error) error {
	br := bufio.NewReaderSize(r, 1024*1024)
	bytes := make([]byte, 16*RowsToRead)
	count, e := br.Read(bytes)
	var (
		key          uint64
		offset, size uint32
		i            int
	)

	for count > 0 && e == nil {
		for i = 0; i+16 <= count; i += 16 {
			key = util.BytesToUint64(bytes[i : i+8])
			offset = util.BytesToUint32(bytes[i+8 : i+12])
			size = util.BytesToUint32(bytes[i+12 : i+16])
			if e = fn(key, offset, size); e != nil {
				return e
			}
		}
		if count%16 != 0 {
			copy(bytes[:count-i], bytes[i:count])
			i = count - i
			count, e = br.Read(bytes[i:])
			count += i
		} else {
			count, e = br.Read(bytes)
		}
	}
	if e == io.EOF {
		return nil
	}
	return e
}

func (nm *NeedleMap) Put(key uint64, offset uint32, size uint32) (int, error) {
	oldSize := nm.m.Set(Key(key), offset, size)
	util.Uint64toBytes(nm.bytes[0:8], key)
	util.Uint32toBytes(nm.bytes[8:12], offset)
	util.Uint32toBytes(nm.bytes[12:16], size)
	nm.FileCounter++
	nm.FileByteCounter = nm.FileByteCounter + uint64(size)
	if oldSize > 0 {
		nm.DeletionCounter++
		nm.DeletionByteCounter = nm.DeletionByteCounter + uint64(oldSize)
	}
	return nm.indexFile.Write(nm.bytes)
}
func (nm *NeedleMap) Get(key uint64) (element *NeedleValue, ok bool) {
	element, ok = nm.m.Get(Key(key))
	return
}
func (nm *NeedleMap) Delete(key uint64) error {
	nm.DeletionByteCounter = nm.DeletionByteCounter + uint64(nm.m.Delete(Key(key)))
	offset, err := nm.indexFile.Seek(0, 1)
	if err != nil {
		return fmt.Errorf("cannot get position of indexfile: %s", err)
	}
	util.Uint64toBytes(nm.bytes[0:8], key)
	util.Uint32toBytes(nm.bytes[8:12], 0)
	util.Uint32toBytes(nm.bytes[12:16], 0)
	if _, err = nm.indexFile.Write(nm.bytes); err != nil {
		plus := ""
		if e := nm.indexFile.Truncate(offset); e != nil {
			plus = "\ncouldn't truncate index file: " + e.Error()
		}
		return fmt.Errorf("error writing to indexfile %s: %s%s", nm.indexFile, err, plus)
	}
	nm.DeletionCounter++
	return nil
}
func (nm *NeedleMap) Close() {
	_ = nm.indexFile.Close()
}
func (nm NeedleMap) ContentSize() uint64 {
	return nm.FileByteCounter
}
func (nm NeedleMap) DeletedSize() uint64 {
	return nm.DeletionByteCounter
}
func (nm NeedleMap) FileCount() int {
	return nm.FileCounter
}
func (nm NeedleMap) DeletedCount() int {
	return nm.DeletionCounter
}
func (nm *NeedleMap) Visit(visit func(NeedleValue) error) (err error) {
	return nm.m.Visit(visit)
}
func (nm NeedleMap) MaxFileKey() uint64 {
	return nm.MaximumFileKey
}
func (nm NeedleMap) NextFileKey(count int) (ret uint64) {
	if count <= 0 {
		return 0
	}
	ret = nm.MaximumFileKey
	nm.MaximumFileKey += uint64(count)
	return
}
