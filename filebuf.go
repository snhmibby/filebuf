package main

/* TODO:
 * - figure out some caching scheme to not read from the file too much
 * - maybe allow for combining nodes if possible
 */

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
)

type FileBuf interface {
	ReadBuf(at int64, size int64) bytes.Buffer
	Size() int64
	Dump()
	//Remove(at int64, length int64)
	//Cut(at int64, length int64) FileBuf
	//Copy(at int64, length int64) FileBuf
	//Paste(at int64, buf *FileBuf)
	InsertBytes(at int64, bs []byte)
	//InsertByte(at int64, b byte)
}

type FileBuffer struct {
	root *Tree
}

func newMemBuffer(b []byte) FileBuf {
	d := newBufData(b)
	t := newTree(d)
	return &FileBuffer{root: t}
}

func newFileBuffer(f string) (FileBuf, error) {
	d, err := newFileData(f)
	if err != nil {
		return nil, err
	}
	r := newTree(d)
	return &FileBuffer{root: r}, nil
}

func (fb *FileBuffer) ReadBuf(at int64, size int64) bytes.Buffer {
	var buf bytes.Buffer
	log.Fatal("TODO: implement FileBuffer.ReadBuf")
	return buf
}

func (fb *FileBuffer) Size() int64 {
	return fb.root.size
}

func (fb *FileBuffer) Dump() {
	for n := fb.root.first(); n != nil; n = n.next() {
		dunkdata(n)
	}
}

func (fb *FileBuffer) InsertBytes(at int64, bs []byte) {
	node, off := fb.root.get(at)
	splay(node)
	fb.root = node

	//split node if we can't append directly
	if off != node.data.Size() {
		ldata, rdata := node.data.Split(off)
		l := newTree(ldata)
		r := newTree(rdata)
		l.setLeft(node.left)
		r.setRight(node.right)
		l.setRight(r)
		fb.root = l
	}
	//make sure we can append to the root node
	if !fb.root.data.Appendable() {
		data := newBufData([]byte{})
		l_ := newTree(data)
		l_.setRight(fb.root.right)
		fb.root.setRight(nil)
		l_.setLeft(fb.root)
		fb.root = l_
		fb.root.resetSize()
	}
	fb.root.data.AppendBytes(bs)
	fb.root.resetSize()
}

type Data interface {
	io.ReaderAt
	Size() int64
	Appendable() bool
	AppendByte(b byte) //these functions are for editing
	AppendBytes(b []byte)
	Split(at int64) (Data, Data)
}

//implements Data interface
type bufData struct {
	data []byte
}

func newBufData(b []byte) *bufData {
	return &bufData{b}
}

func (buf *bufData) ReadAt(p []byte, off int64) (int, error) {
	//?can be implemented with copy(p, buf.data[off:]) or some such?
	var e error = nil
	var psize = len(p)
	var bsize = len(buf.data)
	if int(off) > bsize {
		return 0, io.EOF
	}
	canread := bsize - int(off)
	if psize < canread {
		canread = psize
		e = io.EOF
	}
	copy(p, buf.data[off:])
	return canread, e
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

func (buf *bufData) Split(at int64) (Data, Data) {
	if at > buf.Size() {
		log.Fatal("bufData.Split(): at > len(buf)")
	}
	if at == buf.Size() {
		log.Fatal("bufData.Split(): at = len(buf)")
	}
	newslice := make([]byte, at)
	copy(newslice, buf.data)
	return newBufData(newslice), newBufData(buf.data[at:])
}

//implements Data interface
type fileData struct {
	file   io.ReaderAt
	offset int64
	size   int64
}

func newFileData(fname string) (*fileData, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &fileData{
		file:   f,
		offset: 0,
		size:   stat.Size(),
	}, nil
}

func (f *fileData) ReadAt(p []byte, off int64) (int, error) {
	b := p
	if int64(len(p)) > f.size {
		b = p[:f.size]
	}
	n, err := f.file.ReadAt(b, f.offset+off)
	if int64(n) != f.size {
		err = io.EOF
	}
	return n, err
}

func (f *fileData) Size() int64 {
	return f.size
}

func (f *fileData) Appendable() bool {
	return false
}

func (f *fileData) AppendByte(b byte) {
	log.Fatal("fileData.AppendByte")
}

func (f *fileData) AppendBytes(b []byte) {
	log.Fatal("fileData.AppendBytes")
}

func (f *fileData) Split(at int64) (Data, Data) {
	if at > f.size {
		log.Fatal("fileData.Split: too bug!")
	}

	l := *f
	l.size = at

	r := *f
	r.offset = at
	r.size -= at
	return &l, &r
}

type Tree struct {
	left, right, parent *Tree
	data                Data
	size                int64 //left.size + data.size + right.size
}

func newTree(d Data) *Tree {
	return &Tree{data: d, size: d.Size()}
}

func (t *Tree) setLeft(l *Tree) {
	t.left = l
	if t.left != nil {
		t.left.parent = t
	}
	t.resetSize()
}

func (t *Tree) setRight(r *Tree) {
	t.right = r
	if t.right != nil {
		t.right.parent = t
	}
	t.resetSize()
}

func (t *Tree) setParent(p *Tree) {
	t.parent = p
	if t.parent != nil {
		if p.left != t && p.right != t {
			log.Fatal("setParent(): not a child")
		}
	}
}

func (t *Tree) resetSize() {
	t.size = treesize(t.left) + t.data.Size() + treesize(t.right)
}

func treesize(t *Tree) int64 {
	if t != nil {
		return t.size
	}
	return 0
}

//tree queries
func (node *Tree) first() *Tree {
	n := node
	for n.left != nil {
		n = n.left
	}
	return n
}

func (node *Tree) last() *Tree {
	n := node
	for n.right != nil {
		n = n.right
	}
	return n
}

//return successor of current node.
func (node *Tree) next() *Tree {
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

//return predecessor of current node
func (node *Tree) prev() *Tree {
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

//get the node that contains the requested offset and the offset in this node
func (node *Tree) get(at int64) (*Tree, int64) {
	if at > node.size {
		log.Fatal("Tree.get; at > node.size")
	}
	offsetInNode := at - treesize(node.left)
	nodeSize := node.data.Size()
	switch {
	case offsetInNode < 0:
		return node.left.get(at)
	case offsetInNode <= nodeSize:
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
func rotateLeft(x *Tree) {
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

func rotateRight(x *Tree) {
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
func splay(x *Tree) {
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
}

//Relabel the size fields on the tree
//should be automatic! Only for hand-testing
func resize(n *Tree) int64 {
	if n == nil {
		return 0
	}
	n.size = resize(n.left) + n.data.Size() + resize(n.right)
	return n.size
}

//Set all parents right
//should be automatic! Only for hand-testing
func reparent(n *Tree) {
	if n == nil {
		return
	}
	if n.left != nil {
		n.left.parent = n
		reparent(n.left)
	}
	if n.right != nil {
		n.right.parent = n
		reparent(n.right)
	}
}

func dunkdata(n *Tree) {
	size := n.data.Size()
	b := make([]byte, size)
	read, err := n.data.ReadAt(b, 0)
	if err != nil {
		log.Fatal("dunkdata: whoopsie: ", err)
	}
	if int64(read) != size {
		log.Fatal("dunkdata: read != size???") //is this even possible? no?
	}
	fmt.Print(string(b))
}

func main() {
	fb, _ := newFileBuffer("hellofile.txt")
	fb.InsertBytes(12, []byte("..."))
	fb.InsertBytes(15, []byte("Here i come!!\n"))
	fb.Dump()
	//fb.InsertBytes(0, []byte(":)"))
	//fb.Dump()
	/*
		b := newBufData([]byte("Hello, World"))
		l, r := b.Split(5)
		l.AppendBytes([]byte(" there"))
		r.AppendBytes([]byte(". Here I Come!!"))
		fmt.Println(string(l.data))
		fmt.Println(string(r.data))
	*/

	/*
		f, _ := newFileData("hellofile.txt")
		l, r := f.Split(5)
		fmt.Println(l)
		fmt.Println(r)
	*/

	/*
		var testdata = []*Data{
			newDataFromBuf([]byte("hello")),
			newDataFromBuf([]byte(", ")),
			newDataFromBuf([]byte("world")),
			newDataFromBuf([]byte("!")),
			newDataFromBuf([]byte("\nHere I Come!!\n")),
		}

		hellodata, _ := newDataFromFile("hellofile.txt")
		hellodata.offset = 1
		hellodata.size -= 1

		var leftNode = &Tree{data: hellodata}
		var rightNode = &Tree{data: testdata[2]}
		var aNode = &Tree{left: leftNode, right: rightNode, data: testdata[1]}
		var rrightNode = &Tree{data: testdata[4]}
		var testTree = &Tree{left: aNode, right: rrightNode, data: testdata[3]}
		reparent(testTree)
		resize(testTree)
		for n := testTree.first(); n != nil; n = n.next() {
			//fmt.Println(n)
			dunkdata(n)
		}
	*/
}
