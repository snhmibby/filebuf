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
 * - make Write less inefficient, esp in the case of writing 1 byte
 * - be consistent with panics or returning error
 * - smart writing back to original file (.Save()... operation)
 * - allow for combining nodes if possible
 *   having many small nodes eats memory and grows the tree so everyting bogs down.
 *   having bigger nodes make it a lot faster.
 *   then again splitting a bigger node possibly involves lots of copying
 */

import (
	"fmt"
	"io"
	"sync"
)

//implements io.ReadWriteSeeker
type Buffer struct {
	lock   sync.Mutex //very coarse thread safety
	root   *node
	offset int64 //to implement io.ReaderSeeker
}

func NewEmpty() *Buffer {
	return &Buffer{root: mkNode(mkBuf([]byte{}))}
}

//Use byte array b as source for a filebuffer
func NewMem(b []byte) *Buffer {
	return &Buffer{root: mkNode(mkBuf(b))}
}

//Open file 'f' as source for a filebuffer
//As long as you are using buffers predicated on 'f',
//you probably shouldn't change the file on disk
func OpenFile(f string) (*Buffer, error) {
	d, err := mkFileBuf(f)
	if err != nil {
		return nil, err
	}
	return &Buffer{root: mkNode(d)}, nil
}

/*
 * Simple Thread Safe Interface
 */

func (fb *Buffer) Size() int64 {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	return fb.size()
}

//io.Seeker
func (fb *Buffer) Seek(offset int64, whence int) (int64, error) {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	return fb.seek(offset, whence)
}

//io.Writer
func (fb *Buffer) Write(p []byte) (int, error) {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	return fb.write(p)
}

//io.Reader
func (fb *Buffer) Read(p []byte) (int, error) {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	return fb.read(p)
}

//Dump contents to out
func (fb *Buffer) Dump(out io.Writer) {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	fb.dump(out)
}

func (fb *Buffer) Remove(offset int64, size int64) {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	fb.remove(offset, size)
}

//Cut size bytes at offset
func (fb *Buffer) Cut(offset int64, size int64) *Buffer {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	return fb.cut(offset, size)
}

//Copy size bytes at offset
func (fb *Buffer) Copy(offset int64, size int64) *Buffer {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	return fb.copy(offset, size)
}

//Paste buf at offset (copies the paste buffer)
func (fb *Buffer) Paste(offset int64, paste *Buffer) {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	fb.paste(offset, paste)
}

//Insert a byte slice
func (fb *Buffer) Insert(offset int64, bs []byte) error {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	return fb.insert(offset, bs)
}

//Insert 1 byte
func (fb *Buffer) Insert1(offset int64, b byte) error {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	return fb.insert1(offset, b)
}

//iterate over the file, give the callback byte slices for READING ONLY
func (fb *Buffer) Iter(cb func([]byte) bool) {
	fb.IterFrom(0, cb)
}

//Same as Iter, but start at offset
func (fb *Buffer) IterFrom(from int64, cb func([]byte) bool) {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	fb.iterFrom(from, cb)
}

/*
 * interface implementation
 */
func (fb *Buffer) seek(offset int64, whence int) (int64, error) {
	var newoff int64
	switch {
	case whence == io.SeekStart:
		newoff = offset
	case whence == io.SeekCurrent:
		newoff = fb.offset + offset
	case whence == io.SeekEnd:
		newoff = fb.size() + offset
	}
	if newoff < 0 || newoff > fb.size() {
		//actually fb.offset > fb.size() should be legal, but meh
		return fb.offset, fmt.Errorf("FileBuffer.Seek() bad offset (%d)", newoff)
	}
	fb.offset = newoff
	return fb.offset, nil
}

//The size of the FileBuffer in bytes
func (fb *Buffer) size() int64 {
	return fb.root.size
}

func (fb *Buffer) write(p []byte) (int, error) {
	plen := int64(len(p))

	if plen+fb.offset < fb.size() {
		fb.remove(fb.offset, plen)
	} else {
		if fb.offset > fb.size() {
			return 0, fmt.Errorf("FileBuffer.Write: Attempt to write past EOF")
		} else if fb.offset < fb.size() {
			fb.remove(fb.offset, fb.size()-fb.offset)
		}
	}
	err := fb.insert(fb.offset, p)
	fb.offset += int64(len(p))
	return len(p), err
}

func (fb *Buffer) read(p []byte) (int, error) {
	var err error
	if fb.offset >= fb.size() {
		return 0, io.EOF
	}
	var off int64

	//read root once, then iter down the right subtree
	newroot, off := fb.root.get(fb.offset)
	fb.root = splay(newroot)
	read, err := fb.root.data.ReadAt(p, off)

	if err == nil && read < len(p) {
		fb.root.right.iter(func(t *node) bool {
			var n int
			n, err = t.data.ReadAt(p[read:], 0)
			read += n
			return err != nil || read >= len(p)
		})
	}

	if read == 0 && read < len(p) && err == nil {
		panic("*Buffer.Read didn't read enough??")
	} else {
		fb.offset += int64(read)
	}
	return read, err
}

func (fb *Buffer) dump(out io.Writer) {
	fb.root.iter(func(t *node) bool {
		n, err := t.data.WriteTo(out)
		return err != nil || n != t.data.Size()
	})
}

//Remove size bytes at offset
func (fb *Buffer) remove(offset int64, size int64) {
	fb.cut(offset, size)
}

func (fb *Buffer) cut(offset int64, size int64) *Buffer {
	if offset < 0 || offset > fb.size() || fb.size() < offset+size {
		panic("FileBuffer.Cut: bad offset")
	}

	if size == 0 {
		return NewEmpty()
	}

	fb.findBefore(offset)
	cut := &Buffer{root: fb.root.right}
	cut.root.setParent(nil)
	cut.findBefore(size)
	fb.root.setRight(cut.root.right)
	cut.root.setRight(nil)
	return cut
}
func (fb *Buffer) copy(offset int64, size int64) *Buffer {
	if offset < 0 || offset > fb.size() || fb.size() < offset+size {
		panic("FileBuffer.Copy(): offset or size out of bounds")
	}
	tmpCut := fb.cut(offset, size)
	cpy := &Buffer{root: tmpCut.root.Copy()}
	fb.destuctivePaste(offset, tmpCut)
	return cpy
}

//"destructive join" destuctivePaste buffer into fb
func (fb *Buffer) destuctivePaste(offset int64, paste *Buffer) {
	fb.findBefore(offset)
	extra := fb.root.right
	fb.root.setRight(paste.root)
	fb.root = splay(fb.root.last())
	fb.root.setRight(extra)
}
func (fb *Buffer) paste(offset int64, paste *Buffer) {
	if paste != nil && paste.Size() > 0 {
		p := Buffer{root: paste.root.Copy()}
		fb.destuctivePaste(offset, &p)
	}
}

//Make the root node start exactly at offset (if possible)
//0 <= offset <= fb.size()
func (fb *Buffer) find(offset int64) {
	if offset < 0 {
		panic("FileBuffer.find(): offset < 0")
	} else if offset > fb.size() {
		panic("FileBuffer.find(): offset > filesize")
	}
	node, nodeOffset := fb.root.get(offset)
	fb.root = splay(node)
	if nodeOffset != 0 {
		//Need to split this node
		ldata, rdata := fb.root.data.Split(nodeOffset)
		l := mkNode(ldata)
		r := mkNode(rdata)
		l.setLeft(fb.root.left)
		r.setRight(fb.root.right)
		r.setLeft(l)
		fb.root = r
	}
}

//Set the root node to one that ends at offset-1
//i.e. appending to the root node would insert at offset
func (fb *Buffer) findBefore(offset int64) {
	var before *node
	if offset >= fb.size() {
		before = fb.root.last()
	} else {
		fb.find(offset)
		if fb.root.left != nil {
			before = fb.root.left.last()
		}
	}
	if before == nil {
		before = mkNode(mkBuf([]byte{}))
		fb.root.setLeft(before)
	}
	fb.root = splay(before)
}

func (fb *Buffer) insert(offset int64, bs []byte) error {
	if offset < 0 {
		return fmt.Errorf("FileBuffer.Insertbytes offset < 0")
	} else if offset > fb.size() {
		return fmt.Errorf("FileBuffer.Insertbytes(): offset > FileBuffer.Size()")
	}

	fb.findBefore(offset)
	fb.makeAppendable()
	fb.root.data.AppendBytes(bs)
	fb.root.resetSize()
	return nil
}

func (fb *Buffer) insert1(offset int64, b byte) error {
	if offset < 0 {
		return fmt.Errorf("FileBuffer.Insertbyte offset < 0")
	} else if offset > fb.size() {
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
		data := mkBuf([]byte{})
		newnode := mkNode(data)
		newnode.setRight(fb.root.right)

		//this order is important because .set* functions do size updates
		fb.root.setRight(nil)
		newnode.setLeft(fb.root)

		fb.root.resetSize()
		fb.root = newnode

	}
}

func (fb *Buffer) stats(name string) {
	var st stats
	st.minsz = fb.size() + 1
	fb.root.stats(&st, 0)
	fmt.Printf("\n----- STATS FOR BUFFER %s\nsize = %d\n", name, st.size)
	fmt.Printf("stats.numnodes=%d (file: %d, data: %d (fixed: %d))\n", st.numnodes, st.filenodes, st.datanodes, st.fixeddata)
	fmt.Printf("avg node size: %f (min: %d, max: %d)\n", st.avgsz, st.minsz, st.maxsz)
	fmt.Printf("maxdepth: %d (avg: %f)\n", st.maxdist, st.avgdist)
}

func (fb *Buffer) iterFrom(from int64, cb func([]byte) bool) {
	fb.root.iter(func(n *node) bool {
		var stop = false
		switch n.data.(type) {
		case *fileData:
			//if region is big, split into chunks
			f := n.data.(*fileData)
			var done int64 = 0
			buf := make([]byte, maxBufLen)
			for !stop && done < f.size {
				if f.size-done < maxBufLen {
					buf = buf[:f.size-done]
				}
				n, err := f.file.ReadAt(buf, f.offset+done)
				done += int64(n)
				stop = cb(buf[:n])
				if err != nil {
					stop = stop || done != f.size
				}
			}
		case *bufData:
			stop = cb(n.data.(*bufData).data)
		}
		return stop
	})
}
