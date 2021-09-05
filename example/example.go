package main

import (
	"io"

	"github.com/snhmibby/filebuf"
)

//TODO: write tests

func main() {
	var fb *filebuf.FileBuffer
	//grow a big file by repeated pasting
	/*
		fb = filebuf.NewMemBuffer([]byte("Hello World.\n"))
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

	fb, _ = filebuf.NewFileBuffer("hellofile.txt")
	//replace World by Hacks
	fb.Seek(-6, io.SeekEnd)
	fb.Write([]byte("Hacks"))
	fb.Dump()

	/*
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
	*/

	/*
		//GIGAFILE is a few copies of the repeated "Hello World." above
		//it is about 6GB
		const szhello int64 = int64(len("Hello World.\n"))
		const szcome int64 = int64(len("Here I Come!\n."))
		buf := make([]byte, szhello)
		fb, _ = filebuf.NewFileBuffer("GIGAFILE")
		fb.Read(buf)
		os.Stdout.Write(buf)
		fb.Seek(-szcome, io.SeekEnd)
		io.Copy(os.Stdout, fb)
	*/

}
