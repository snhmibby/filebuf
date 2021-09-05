//Package for efficient editing operations on big files
package filebuf

/* TODO:
 * - figure out some caching scheme to not read from the file too much
 *   maybe just mmap whole files?
 *   or mmap the file in chunks
 *   either way, mmap seems a nice solution because it offloads the caching and
 *   swapping out problem to the OS Golang doesn't have weak references or some
 *   such so the GC can't be utilized for swapping chunks out
 * - allow for combining nodes if possible
 *   having many small nodes eats memory and grows the tree so everyting bogs down.
 *   having bigger nodes make it a lot faster.
 *   then again splitting a bigger node possibly involves lots of copying
 */

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"golang.org/x/exp/mmap"
)

//implements io.ReadWriteSeeker
type FileBuffer struct {
	root   *tree
	offset int64 //to implement io.ReaderSeeker
}

//Use byte array b as source for a filebuffer
func NewMemBuffer(b []byte) *FileBuffer {
	d := newBufData(b)
	t := newTree(d)
	return &FileBuffer{root: t}
}

//Open file 'f' as source for a filebuffer
//As long as you are using buffers predicated on 'f',
//you probably shouldn't change the file on disk
func NewFileBuffer(f string) (*FileBuffer, error) {
	d, err := newFileData(f)
	if err != nil {
		return nil, err
	}
	r := newTree(d)
	return &FileBuffer{root: r}, nil
}

//The size of the FileBuffer in bytes
func (fb *FileBuffer) Size() int64 {
	return fb.root.size
}

//io.Seeker
func (fb *FileBuffer) Seek(offset int64, whence int) (int64, error) {
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
func (fb *FileBuffer) Write(p []byte) (int, error) {
	if fb.offset > fb.Size() {
		return 0, io.EOF
	}
	fb.Remove(fb.offset, int64(len(p)))
	fb.InsertBytes(fb.offset, p)
	return len(p), nil
}

//io.Reader
func (fb *FileBuffer) Read(p []byte) (int, error) {
	var err error

	if fb.offset >= fb.Size() {
		return 0, io.EOF
	}

	fb.find(fb.offset)
	toread := len(p)
	read := 0
	node := fb.root
	for node != nil && toread > 0 {
		n, _ := node.data.ReadAt(p[read:], 0)
		read += n
		toread -= n
		node = node.next()
	}
	if node == nil {
		err = io.EOF
	}
	fb.offset += int64(read)
	return read, err
}

//Read size bytes starting at position at
func (fb *FileBuffer) ReadBuf(offset int64, size int64) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	oldoffset := fb.offset
	fb.offset = offset
	_, err := io.CopyN(&buf, fb, size)
	fb.offset = oldoffset
	return &buf, err
}

//Dump contents to stdout
func (fb *FileBuffer) Dump() {
	for n := fb.root.first(); n != nil; n = n.next() {
		b := make([]byte, n.data.Size())
		n.data.ReadAt(b, 0)
		fmt.Print(string(b))
	}
	fmt.Println()
}

//Remove size bytes at offset
func (fb *FileBuffer) Remove(offset int64, size int64) {
	fb.Cut(offset, size)
}

//Cut size bytes at offset
func (fb *FileBuffer) Cut(offset int64, size int64) *FileBuffer {
	fb.findBefore(offset)
	cut := &FileBuffer{root: fb.root.right}
	cut.root.setParent(nil)
	cut.findBefore(size)
	fb.root.setRight(cut.root.right)
	cut.root.setRight(nil)
	return cut
}

//Copy size bytes at offset
func (fb *FileBuffer) Copy(offset int64, size int64) *FileBuffer {
	tmpCut := fb.Cut(offset, size)
	cpy := &FileBuffer{root: tmpCut.root.Copy()}
	fb.Paste(offset, tmpCut)
	return cpy
}

//Paste buf at offset
func (fb *FileBuffer) Paste(offset int64, paste *FileBuffer) {
	fb.findBefore(offset)
	extra := fb.root.right
	newtree := paste.root.Copy()
	fb.root.setRight(newtree)
	fb.root = splay(fb.root.last())
	fb.root.setRight(extra)
}

//0 <= offset <= fb.Size()
func (fb *FileBuffer) find(offset int64) {
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

//Set the root node to one that ends at offset (so we can append to it)
func (fb *FileBuffer) findBefore(offset int64) {
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

func (fb *FileBuffer) InsertBytes(offset int64, bs []byte) error {
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

func (fb *FileBuffer) InsertByte(offset int64, b byte) error {
	if offset < 0 {
		return fmt.Errorf("FileBuffer.Insertbytes offset < 0")
	} else if offset > fb.Size() {
		return fmt.Errorf("FileBuffer.Insertbytes(): offset > FileBuffer.Size()")
	}

	fb.findBefore(offset)
	fb.makeAppendable()
	fb.root.data.AppendByte(b)
	fb.root.resetSize()
	return nil
}

//Make the root node appendable, insert a new []buffer node if necessary
func (fb *FileBuffer) makeAppendable() {
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

/***************************************************************************************
 * Data is an interface for a piece of data that comes from a certain source
 * For now we have 2 sources, a memory buffer ([]byte) or a file.
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
	data []byte
}

func newBufData(b []byte) *bufData {
	return &bufData{b}
}

func (buf *bufData) ReadAt(p []byte, off int64) (int, error) {
	var bsize = len(buf.data)
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
	return true
}

func (buf *bufData) AppendByte(b byte) {
	buf.data = append(buf.data, b)
}

func (buf *bufData) AppendBytes(b []byte) {
	buf.data = append(buf.data, b...)
}

func (buf *bufData) Split(offset int64) (data, data) {
	if offset > buf.Size() {
		panic("bufData.Split(): offset > len(buf)")
	}
	if offset == buf.Size() {
		panic("bufData.Split(): offset = len(buf)")
	}
	newslice := make([]byte, offset)
	copy(newslice, buf.data)
	return newBufData(newslice), newBufData(buf.data[offset:])
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
	if int64(len(p)) > f.size {
		b = p[:f.size]
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

	l := *f
	l.size = offset

	r := *f
	r.offset = offset
	r.size -= offset
	return &l, &r
}

func (f *fileData) Copy() data {
	return f
}

/* A binary tree that holds Data */
type tree struct {
	left, right, parent *tree
	data                data
	size                int64 //left.size + data.size + right.size
}

func newTree(d data) *tree {
	return &tree{data: d, size: d.Size()}
}

//Copy this tree
func (t *tree) Copy() *tree {
	n := *t
	n.data = n.data.Copy()
	if n.left != nil {
		n.left = n.left.Copy()
		n.left.parent = &n
	}
	if n.right != nil {
		n.right = n.right.Copy()
		n.right.parent = &n
	}
	return &n
}

/* The set{Left, Right, Parent} functions should be used,
 * because they take into account updating the size field */

func (t *tree) setLeft(l *tree) {
	t.left = l
	if t.left != nil {
		t.left.parent = t
	}
	t.resetSize()
}

func (t *tree) setRight(r *tree) {
	t.right = r
	if t.right != nil {
		t.right.parent = t
	}
	t.resetSize()
}

func (t *tree) setParent(p *tree) {
	t.parent = p
	if t.parent != nil {
		t.parent.resetSize()
	}
}

func (t *tree) resetSize() {
	t.size = treesize(t.left) + t.data.Size() + treesize(t.right)
}

//helper function to query t.size, return 0 on t == nil
func treesize(t *tree) int64 {
	if t != nil {
		return t.size
	}
	return 0
}

func (node *tree) first() *tree {
	n := node
	for n.left != nil {
		n = n.left
	}
	return n
}

func (node *tree) last() *tree {
	n := node
	for n.right != nil {
		n = n.right
	}
	return n
}

func (node *tree) next() *tree {
	n := node
	if n.right != nil {
		n = n.right.first()
	} else {
		for n.parent != nil && n.parent.right == n {
			n = n.parent
		}
		n = n.parent
	}
	return n
}

func (node *tree) prev() *tree {
	n := node
	if n.left != nil {
		n = n.left.last()
	} else {
		for n.parent != nil && n.parent.left == n {
			n = n.parent
		}
		n = n.parent
	}
	return n
}

//get the node that contains the requested offset
func (node *tree) get(offset int64) (*tree, int64) {
	if offset > node.size {
		panic("tree.get; offset > node.size")
	}
	offsetInNode := offset - treesize(node.left)
	nodeSize := node.data.Size()
	switch {
	case offsetInNode < 0:
		return node.left.get(offset)
	case offsetInNode < nodeSize:
		return node, offsetInNode
	default:
		return node.right.get(offsetInNode - nodeSize)
	}
}

//splay functions from wikipedia
//take care to adjust the size fields

/* Cool ansi art illustration:
 *                        y
 *         x             / \
 *        / \    -->    x   c
 *       a   y         / \
 *          / \       a   b
 *         b   c
 */
func rotateLeft(x *tree) {
	y := x.right
	if y != nil {
		x.setRight(y.left)
		y.setParent(x.parent)
	}
	if x.parent == nil {
		//?
	} else if x == x.parent.left {
		x.parent.setLeft(y)
	} else {
		x.parent.setRight(y)
	}
	if y != nil {
		y.setLeft(x)
	}
	x.setParent(y)
}

/* Cool ansi art illustration:
 *                        x
 *         y             / \
 *        / \    <--    y   c
 *       a   x         / \
 *          / \       a   b
 *         b   c
 */
func rotateRight(x *tree) {
	y := x.left
	if y != nil {
		x.setLeft(y.right)
		y.setParent(x.parent)
	}
	if x.parent == nil {
	} else if x == x.parent.right {
		x.parent.setRight(y)
	} else {
		x.parent.setLeft(y)
	}
	if y != nil {
		y.setRight(x)
	}
	x.setParent(y)
}

//see https://en.wikipedia.org/wiki/Splay_tree
func splay(x *tree) *tree {
	for x.parent != nil {
		if x.parent.parent == nil {
			if x == x.parent.left {
				rotateRight(x.parent)
			} else {
				rotateLeft(x.parent)
			}
		} else if x.parent.left == x && x.parent.parent.left == x.parent {
			rotateRight(x.parent.parent)
			rotateRight(x.parent)
		} else if x.parent.right == x && x.parent.parent.right == x.parent {
			rotateLeft(x.parent.parent)
			rotateLeft(x.parent)
		} else if x.parent.left == x && x.parent.parent.right == x.parent {
			rotateRight(x.parent)
			rotateLeft(x.parent)
		} else {
			rotateLeft(x.parent)
			rotateRight(x.parent)
		}
	}
	return x
}
