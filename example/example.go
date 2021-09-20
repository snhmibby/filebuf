package main

import (
	"bytes"
	"io"
	"log"
	"os"

	B "github.com/snhmibby/filebuf"
)

const testfile_path = "TESTFILE"

func emptyTestFile() *B.FileBuffer {
	os.Create(testfile_path)
	b, err := B.NewFileBuffer(testfile_path)
	if err != nil {
		log.Fatal(`NewFileBuffer("TESTFILE") failed, err = `, err)
		return nil
	}
	return b
}

var testdata = []byte(`Hello World!
this is some testdata.
this is the third line.
`)

func main() {
	var fb *B.FileBuffer
	/*
		//grow a big file by repeated pasting
		fb = B.NewMemBuffer([]byte("Hello World.\n"))
		for i := 0; i < 20; i++ {
			fb.Paste(0, fb)
		}
		//this creates 1 big byte array which we can copy multiple times
		b, _ := io.ReadAll(fb)
		for i := 0; i < 20; i++ {
			fb.InsertBytes(0, b)
		}
		b, _ = io.ReadAll(fb)
		for i := 0; i < 5; i++ {
			fb.InsertBytes(0, b)
		}
		fb.InsertBytes(fb.Size(), []byte("\nHere I Come!\n")) //append something
		fb.Dump()
	*/

	/*
		fb, _ = B.NewFileBuffer("hellofile.txt")
		//replace World by Hacks
		fb.Seek(6, io.SeekStart)
		fb.Write([]byte("Hacks"))
		c := fb.Cut(7, 5)
		c.Dump()
		fmt.Println()
		fb.Dump()
	*/

	/*
		fb = B.NewMemBuffer([]byte("abc"))
		c := fb.Cut(1, 1)
		fb.Dump()
		c.Dump()
		fb.Paste(1, c)
		fb.Dump()
		c = fb.Cut(1, 2)
		fb.Dump()
		c.Dump()
	*/
	//fb = B.NewMemBuffer([]byte("Hello, World!"))
	/*
		fb, _ = B.NewFileBuffer("hellofile.txt")
		hello := fb.Cut(0, 5)
		world := fb.Cut(2, 6)
		hello.Dump()
		world.Dump()

		hw := B.NewMemBuffer([]byte{})
		hw.Paste(0, hello)
		hw.Paste(6, world)
		hw.InsertBytes(5, []byte(", "))
		hw.Remove(4, 8)
		hw.Dump()

	*/

	/*
		fb, _ = B.NewFileBuffer("hellofile.txt")
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
	*/

	/*
		//GIGAFILE is a few copies of the repeated "Hello World." above
		//it is about 6GB
		const szhello int64 = int64(len("Hello World.\n"))
		const szcome int64 = int64(len("Here I Come!\n."))
		buf := make([]byte, szhello)
		fb, err := B.NewFileBuffer("GIGAFILE")
		if err != nil {
			log.Fatal("example.go: Couldn't create filebuffer: ", err)
		}
		fb.Read(buf)
		os.Stdout.Write(buf)
		fb.Seek(-szcome, io.SeekEnd)
		io.Copy(os.Stdout, fb)
		io.Copy(os.Stdout, fb)
	*/
	fb = emptyTestFile()
	if fb == nil {
		log.Fatal(`NewFileBuffer("TESTFILE"), no error but is 'nil'`)
	}
	for i := 0; i < len(testdata); i++ {
		n, err := fb.Write(testdata[i:+1])
		if err != nil || n != 1 {
			log.Fatal("TestNewMemBuf(): couldn't Write():")
		}
	}

	testfile2 := testfile_path + "2"
	f, err := os.Open(testfile2)
	if err != nil {
		log.Fatal("Couldn't open ", testfile2)
	}
	io.Copy(f, fb)
	f.Seek(0, io.SeekStart)
	data, err := io.ReadAll(f)
	if err != nil {
		log.Fatal("Couldn't write ", testfile2)
	}
	if !bytes.Equal(data, testdata) {
		log.Fatal("testfile != testfile2")
	}

}
