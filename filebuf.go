//Package for efficient editing operations on big files
package filebuf

/* A FileBuffer maintains a representation of a buffer in a splay tree,
   where each node in the tree represents a portion of the buffer.
   The data in a node can be either a portion of a file or a byte slice.
   Cut, Copy and Paste operations thus only copy a tree, not an entire slice.
   Insert becomes (possibly) splitting a node and appending to a slice.

   Saving to a file becomes a bit cumbersome to do efficiently and is not implemented (yet).
   You can io.Copy the buffer to a temporary file and rename it to the original.
   This serializes the entire buffer and is not necessary and slow :((
*/

/* TODO:
 * - be consistent with panics or returning error
 * - smart writing back to original file (.Save()... operation)
 * - maintain undo/redo queue
 * - string/regex searching
 * - allow for combining nodes if possible
 *   having many small nodes eats memory and grows the tree so everyting bogs down.
 *   having bigger nodes make it a lot faster.
 *   then again splitting a bigger node possibly involves lots of copying
 */

import (
	"fmt"
	"io"
)

//implements io.ReadWriteSeeker
type Buffer struct {
	root   *tree
	offset int64 //to implement io.ReaderSeeker
}

func NewEmpty() *Buffer {
	t := newTree(newBufData([]byte{}))
	return &Buffer{root: t}
}

//Use byte array b as source for a filebuffer
func NewMem(b []byte) *Buffer {
	d := newBufData(b)
	t := newTree(d)
	return &Buffer{root: t}
}

//Open file 'f' as source for a filebuffer
//As long as you are using buffers predicated on 'f',
//you probably shouldn't change the file on disk
func OpenFile(f string) (*Buffer, error) {
	d, err := newFileData(f)
	if err != nil {
		return nil, err
	}
	r := newTree(d)
	return &Buffer{root: r}, nil
}

//The size of the FileBuffer in bytes
func (fb *Buffer) Size() int64 {
	return fb.root.size
}

//io.Seeker
func (fb *Buffer) Seek(offset int64, whence int) (int64, error) {
	switch {
	case whence == io.SeekStart:
		fb.offset = offset
	case whence == io.SeekCurrent:
		fb.offset += offset
	case whence == io.SeekEnd:
		fb.offset = fb.Size() + offset
	}
	if fb.offset < 0 || fb.offset > fb.Size() {
		//actually fb.offset > fb.Size() should be legal, but meh
		return fb.offset, fmt.Errorf("FileBuffer.Seek() bad offset")
	}
	return fb.offset, nil
}

//io.Writer
func (fb *Buffer) Write(p []byte) (int, error) {
	plen := int64(len(p))

	if plen+fb.offset < fb.Size() {
		fb.Remove(fb.offset, plen)
	} else {
		//at least partial write over EOF?
		if fb.offset > fb.Size() {
			return 0, fmt.Errorf("FileBuffer.Write: Attempt to write past EOF")
		} else if fb.offset < fb.Size() {
			fb.Remove(fb.offset, fb.Size()-fb.offset)
		}
	}
	err := fb.Insert(fb.offset, p)
	fb.offset += int64(len(p))
	return len(p), err
}

func min(x, y int) int {
	if x > y {
		return y
	} else {
		return x
	}
}

//io.Reader
func (fb *Buffer) Read(p []byte) (int, error) {
	//XXX this is messy
	var err error
	if fb.offset >= fb.Size() {
		return 0, io.EOF
	}
	fb.find(fb.offset)
	toread := len(p)
	read := 0
	node := fb.root
	for node != nil && toread > 0 {
		var n int
		nodesize := int(node.data.Size())
		canread := min(nodesize, toread)
		b := p[read : read+canread]
		n, err = node.data.ReadAt(b, 0)
		read += n
		toread -= n
		if n != canread || err != nil {
			break
		}
		node = node.next()
	}
	if read == 0 && read < len(p) {
		err = io.EOF
	} else {
		fb.offset += int64(read)
	}
	return read, err
}

//Dump contents to out
func (fb *Buffer) Dump(out io.Writer) error {
	for n := fb.root.first(); n != nil; n = n.next() {
		b := make([]byte, n.data.Size())
		n.data.ReadAt(b, 0)
		n, err := out.Write(b)
		if err != nil || n != len(b) {
			return err
		}
	}
	return nil
}

//Remove size bytes at offset
func (fb *Buffer) Remove(offset int64, size int64) {
	fb.Cut(offset, size)
}

//Cut size bytes at offset
func (fb *Buffer) Cut(offset int64, size int64) *Buffer {
	if offset < 0 || offset > fb.Size() || fb.Size() < offset+size {
		panic("FileBuffer.Cut: offsets OOB")
	}

	if size == 0 {
		return NewMem([]byte{})
	}

	fb.findBefore(offset)
	cut := &Buffer{root: fb.root.right}
	cut.root.setParent(nil)
	cut.findBefore(size)
	fb.root.setRight(cut.root.right)
	cut.root.setRight(nil)
	return cut
}

//Copy size bytes at offset
func (fb *Buffer) Copy(offset int64, size int64) *Buffer {
	if offset < 0 || offset > fb.Size() || fb.Size() < offset+size {
		panic("FileBuffer.Copy(): offset or size out of bounds")
	}
	tmpCut := fb.Cut(offset, size)
	cpy := &Buffer{root: tmpCut.root.Copy()}
	fb.paste(offset, tmpCut)
	return cpy
}

//"destructive join" paste buffer into fb
func (fb *Buffer) paste(offset int64, paste *Buffer) {
	fb.findBefore(offset)
	extra := fb.root.right
	fb.root.setRight(paste.root)
	fb.root = splay(fb.root.last())
	fb.root.setRight(extra)
}

//Paste buf at offset (copies the paste buffer)
func (fb *Buffer) Paste(offset int64, paste *Buffer) {
	if paste != nil && paste.Size() > 0 {
		p := *paste
		p.root = p.root.Copy()
		fb.paste(offset, &p)
	}
}

//Make the root node start exactly at offset (if possible)
//0 <= offset <= fb.Size()
func (fb *Buffer) find(offset int64) {
	if offset < 0 {
		panic("FileBuffer.find(): offset < 0")
	} else if offset > fb.Size() {
		panic("FileBuffer.find(): offset > filesize")
	}
	node, nodeOffset := fb.root.get(offset)
	fb.root = splay(node)
	if nodeOffset != 0 {
		//Need to split this node
		ldata, rdata := fb.root.data.Split(nodeOffset)
		l := newTree(ldata)
		r := newTree(rdata)
		l.setLeft(fb.root.left)
		r.setRight(fb.root.right)
		r.setLeft(l)
		fb.root = r
	}
}

//Set the root node to one that ends at offset
//i.e. appending to the root node would insert at offset
func (fb *Buffer) findBefore(offset int64) {
	var before *tree
	if offset >= fb.Size() {
		before = fb.root.last()
	} else {
		fb.find(offset)
		before = fb.root.prev()
	}
	if before == nil {
		before = newTree(newBufData([]byte{}))
		fb.root.setLeft(before)
	}
	fb.root = splay(before)
}

func (fb *Buffer) Insert(offset int64, bs []byte) error {
	if offset < 0 {
		return fmt.Errorf("FileBuffer.Insertbytes offset < 0")
	} else if offset > fb.Size() {
		return fmt.Errorf("FileBuffer.Insertbytes(): offset > FileBuffer.Size()")
	}

	fb.findBefore(offset)
	fb.makeAppendable()
	fb.root.data.AppendBytes(bs)
	fb.root.resetSize()
	return nil
}

func (fb *Buffer) Insert1(offset int64, b byte) error {
	if offset < 0 {
		return fmt.Errorf("FileBuffer.Insertbyte offset < 0")
	} else if offset > fb.Size() {
		return fmt.Errorf("FileBuffer.Insertbyte(): offset > FileBuffer.Size()")
	}

	fb.findBefore(offset)
	fb.makeAppendable()
	fb.root.data.AppendByte(b)
	fb.root.resetSize()
	return nil
}

//Make the root node appendable, insert a new, appendable node if necessary
func (fb *Buffer) makeAppendable() {
	if !fb.root.data.Appendable() {
		data := newBufData([]byte{})
		newnode := newTree(data)
		newnode.setRight(fb.root.right)

		//this order is important because .set* functions do size updates
		fb.root.setRight(nil)
		newnode.setLeft(fb.root)

		fb.root.resetSize()
		fb.root = newnode

	}
}

func (fb *Buffer) Stats(name string) {
	var st Stats
	fb.root.stats(&st, 0)
	if fb.Size() != st.size {
		panic("Stats: sizes don't match")
	}
	fmt.Printf("\n----- STATS FOR BUFFER %s\nsize = %d\n", name, st.size)
	fmt.Printf("stats.numnodes=%d (file: %d, data: %d)\n", st.numnodes, st.filenodes, st.datanodes)
	fmt.Printf("maxdist: %d (avg: %f)\n", st.maxdist, st.avgdist)
	fmt.Printf("avgsize: %f (min: %d, max: %d)\n", st.avgsz, st.minsz, st.maxsz)
}
