package filebuf

import (
	"io"
	"os"

	"golang.org/x/exp/mmap"
)

/***************************************************************************************
 * Data is an interface for a piece of data that comes from a certain source
 * For now we have 2 sources, a memory buffer ([]byte) or a file (io.ReaderAt)
 * TODO: data.Combine(another *Data) *Data {...} kind of functionality
 */
type data interface {
	io.ReaderAt
	Size() int64
	Appendable() bool
	AppendByte(b byte) //these functions are for editing
	AppendBytes(b []byte)
	Split(offset int64) (data, data)
	Copy() data
}

//[]Byte buffered data
type bufData struct {
	data       []byte
	appendable bool //freeze on splitting and after 1kb of data
}

func newBufData(b []byte) *bufData {
	return &bufData{data: b, appendable: true}
}

func newStaticBuf(b []byte) *bufData {
	n := newBufData(b)
	n.appendable = false
	return n
}

func (buf *bufData) ReadAt(p []byte, off int64) (int, error) {
	var bsize = len(buf.data)
	if bsize == 0 {
		return 0, nil
	}
	if int(off) >= bsize {
		return 0, io.EOF
	}
	n := copy(p, buf.data[off:])
	return n, nil
}

func (buf *bufData) Size() int64 {
	return int64(len(buf.data))
}

func (buf *bufData) Appendable() bool {
	return buf.appendable
}

func (buf *bufData) AppendByte(b byte) {
	if !buf.appendable {
		panic("buffer is not appendable")
	}
	buf.data = append(buf.data, b)
	buf.appendable = buf.appendable && len(buf.data) < 1024
}

func (buf *bufData) AppendBytes(b []byte) {
	if !buf.appendable {
		panic("buffer is not appendable")
	}
	buf.data = append(buf.data, b...) //XXX This might be slow?
	buf.appendable = buf.appendable && len(buf.data) < 1024
}

func (buf *bufData) Split(offset int64) (data, data) {
	if offset > buf.Size() {
		panic("bufData.Split(): offset > len(buf)")
	}
	if offset == buf.Size() {
		panic("bufData.Split(): offset = len(buf)")
	}
	/*
		newslice := make([]byte, len(buf.data)-int(offset))
		copy(newslice, buf.data[offset:])
	*/
	return newStaticBuf(buf.data[:offset]), newStaticBuf(buf.data[offset:])
}

func (buf *bufData) Copy() data {
	b := make([]byte, len(buf.data))
	copy(b, buf.data)
	return newBufData(b)
}

//File buffered data
type fileData struct {
	file   io.ReaderAt
	offset int64
	size   int64
}

//it might not be a bad idea to mmap HUGE files on 64bit systems?
//i mean it is 2021, right?
func newFileData(fname string) (*fileData, error) {
	var f fileData
	var use_mmap = true
	if use_mmap {
		file, err := mmap.Open(fname)
		if err != nil {
			return nil, err
		}
		f.file = file
		f.size = int64(file.Len())
	} else {
		file, err := os.Open(fname)
		if err != nil {
			return nil, err
		}
		stat, err := file.Stat()
		if err != nil {
			file.Close()
			return nil, err
		}
		f.file = file
		f.size = stat.Size()
	}
	return &f, nil
}

func (f *fileData) ReadAt(p []byte, off int64) (int, error) {
	b := p
	//bounds checking
	if off > f.size {
		return 0, io.EOF
	} else if int64(len(p)) > f.size-off {
		// limit read buffer to this node size
		b = p[:f.size-off]
	}
	n, err := f.file.ReadAt(b, f.offset+off)
	return n, err
}

func (f *fileData) Size() int64 {
	return f.size
}

func (f *fileData) Appendable() bool {
	return false
}

func (f *fileData) AppendByte(b byte) {
	panic("fileData.AppendByte")
}

func (f *fileData) AppendBytes(b []byte) {
	panic("fileData.AppendBytes")
}

func (f *fileData) Split(offset int64) (data, data) {
	if offset > f.size {
		panic("fileData.Split: offset > f.size")
	}
	var l, r fileData

	l = *f
	l.size = offset

	r = *f
	r.offset += offset
	r.size -= offset
	return &l, &r
}

func (f *fileData) Copy() data {
	return f
}
