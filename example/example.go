package main

import (
	"fmt"
	"io"

	"github.com/snhmibby/filebuf"
)

func main() {
	var fb *filebuf.FileBuffer
	/*
		fb := filebuf.NewMemBuffer([]byte("Hello World."))
		h, _ := fb.Cut(0, 5) //Hello
		h.InsertByte(5, ' ')
		h.Paste(0, h)
		h.Paste(0, h)
		h.Paste(0, h)
		h.Paste(0, h)
		h.Paste(0, h)
		fb.Remove(0, 1)                                       //remove extra space :)
		fb.Paste(0, h)                                        //hello hello hello .... world
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
