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
	Combine(d data) data //combine this node and d, if possible (nil if not)
}

//[]Byte buffered data
type bufData struct {
	data   []byte
	frozen bool //freeze on splitting or after 1kb of data
}

const maxBufLen = 4096

func mkBuf(b_ []byte) *bufData {
	b := make([]byte, len(b_))
	copy(b, b_)
	return &bufData{data: b, frozen: false}
}

func mkStatic(b []byte) *bufData {
	return &bufData{data: b, frozen: true}
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
	return !buf.frozen
}

func (buf *bufData) AppendByte(b byte) {
	if buf.frozen {
		panic("buffer is not appendable")
	}
	buf.data = append(buf.data, b)
	buf.frozen = buf.frozen || len(buf.data) > maxBufLen
}

func (buf *bufData) AppendBytes(b []byte) {
	if buf.frozen {
		panic("buffer is not appendable")
	}
	buf.data = append(buf.data, b...)
	buf.frozen = buf.frozen || len(buf.data) > maxBufLen
}

func (buf *bufData) Split(offset int64) (data, data) {
	if offset > buf.Size() {
		panic("bufData.Split(): offset > len(buf)")
	}
	if offset == buf.Size() {
		panic("bufData.Split(): offset = len(buf)")
	}
	/* setting buffers as 'static' after splitting them saves a copy
	newslice := make([]byte, len(buf.data)-int(offset))
	copy(newslice, buf.data[offset:])
	return NewMem(buf.data[:offset]), NewMem(newslice)
	*/
	return mkStatic(buf.data[:offset]), mkStatic(buf.data[offset:])
}

func (buf *bufData) Copy() data {
	if buf.frozen {
		return buf
	} else {
		return mkBuf(buf.data)
	}
}

func (buf *bufData) Combine(d data) data {
	if d.Size() < maxBufLen && buf.Size() < maxBufLen {
		newbuf := make([]byte, d.Size()+buf.Size())
		copy(newbuf, buf.data)
		d.ReadAt(newbuf[buf.Size():], 0)
		return &bufData{newbuf, false}
	}
	return nil
}

//File buffered data
type fileData struct {
	file   io.ReaderAt
	offset int64
	size   int64
}

//it might not be a bad idea to mmap HUGE files on 64bit systems?
//i mean it is 2021, right?
func mkFileBuf(fname string) (*fileData, error) {
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

func (f *fileData) Combine(d data) data {
	f2, ok := d.(*fileData)
	if !ok {
		return nil
	}
	if f.file == f2.file && f.offset+f.size == f2.offset {
		return &fileData{file: f.file, offset: f.offset, size: f.size + f2.size}
	}
	return nil
}
