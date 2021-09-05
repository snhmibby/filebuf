package main

import (
	"fmt"
	"io"

	"github.com/snhmibby/filebuf"
)

func main() {
	var fb *filebuf.FileBuffer
	/*
		fb = filebuf.NewMemBuffer([]byte("Hello World."))
		h := fb.Cut(0, 5) //Hello
		h.InsertByte(4, 'X')
		h.InsertByte(3, 'X')
		h.InsertByte(2, 'X')
		h.InsertByte(1, 'X')
		//this makes a ridiculous number of very very tiny nodes (1,2 bytes each)
		h.Paste(0, h)
		h.Paste(2, h)
		h.Paste(8, h)
		h.Paste(16, h)
		h.Paste(33, h)
		h.Paste(33, h)
		h.Paste(133, h)
		h.Paste(233, h)
		h.Paste(233, h)
		h.Paste(233, h)
		h.Paste(233, h)
		h.Paste(233, h)
		h.Paste(233, h)
		h.Paste(233, h)
		h.Paste(233, h)
		h.Paste(233, h)
		h.Paste(233, h)
		h.Paste(233, h)
		h.Paste(233, h)
		h.Paste(233, h)
		//this creates 1 big byte array which we can copy multiple times
		b, _ := h.ReadBuf(0, h.Size())
		fb.InsertBytes(0, b.Bytes())
		fb.InsertBytes(0, b.Bytes())
		c := fb.Cut(0, int64(2*b.Len()))
		c.Paste(214224, c)
		c.Paste(3214224, c)
		c.Paste(5214224, c)
		c.Paste(10214224, c)
		fb.Paste(0, c)

		fb.InsertBytes(fb.Size(), []byte("\nHere I Come!\n")) //append something
		fb.Dump()
	*/

	fb, _ = filebuf.NewFileBuffer("hellofile.txt")
	fb.InsertBytes(fb.Size(), []byte(":) Here I come!\n"))
	b, _ := io.ReadAll(fb)
	fmt.Printf("b[%d]:%s\n", len(b), b)

	fb.Seek(13, io.SeekStart)
	b, _ = io.ReadAll(fb)
	fmt.Printf("b[%d]:%s\n", len(b), b)

	fb.Seek(13, io.SeekStart)
	fb.Write([]byte(":("))
	fb.Seek(-fb.Size(), io.SeekEnd)
	b, _ = io.ReadAll(fb)
	fmt.Printf("b[%d]:%s\n", len(b), b)

	/*
		fb, _ := NewFileBuffer("MEGAFILE")
		c, _ := fb.Cut(2208301400, 3)
		fb.InsertBytes(2208301400, []byte("Whoooo"))
		fb.Paste(2208301403, c)
		c, _ = fb.Cut(2208301400, 10)
		c.Dump()

		c, _ = fb.Cut(2, 3)
		fb.InsertBytes(2, []byte("Whoooo"))
		fb.Paste(5, c)
		c, _ = fb.Cut(2, 10)
		c.Dump()
	*/
}
