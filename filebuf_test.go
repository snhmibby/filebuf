package filebuf

/* TODO:
 * - these tests only test working basic functionality and don't try to trigger errors
 *   should be about half/half imo
 */

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"testing"
	"time"
)

const TESTDATA_REPEAT = 5000

var helloworld = []byte("Hello World!\n")
var testdata_line2 = []byte("this is some testdata.\n")
var testdata = []byte(`Hello World!
this is some testdata.
this is the third line.
`)

//compare a filebuf to another file
//XXX this slurps entire file into a buffer; only use small files here
func compareBuf2File(b *FileBuffer, f io.ReadSeeker) bool {
	b.Seek(0, io.SeekStart)
	b_text, err := io.ReadAll(b)
	if err != nil {
		return false
	}

	f.Seek(0, io.SeekStart)
	f_text, err := io.ReadAll(f)
	if err != nil {
		return false
	}

	return bytes.Equal(b_text, f_text)
}

//compare a filebuf to a string
//XXX does a byte-by-byte comparison, only use small files
func compareBuf2Bytes(fb *FileBuffer, s []byte) bool {
	b := make([]byte, 1)
	fb.Seek(0, io.SeekStart)
	for i := 0; i < len(s); i++ {
		fb.Read(b)
		if b[0] != s[i] {
			return false
		}
	}
	return true
}

//create lots of new nodes by adding b byte-per-byte, backwards, to the end of fb
func shittyAppend2Buf(fb *FileBuffer, b []byte) error {
	var e error
	for i := 1; i <= len(b); i++ {
		e = fb.InsertByte(0, b[len(b)-i])
		if e != nil {
			return e
		}
	}
	return nil
}

//add a small bunch of data in a sort of randomized way to fb
//the final buffer should hold testdata, repeated TESTDATA_REPEAT times
func createTestData(fb *FileBuffer) {
	for n := 0; n < TESTDATA_REPEAT; n++ {
		switch n % 5 {
		case 0:
			fb.Seek(0, io.SeekEnd)
			for i := 0; i < len(testdata); i++ {
				fb.Write(testdata[i : i+1])
			}
		case 1:
			fb.InsertBytes(0, testdata)
		case 2:
			shittyAppend2Buf(fb, testdata)
		case 3:
			fb.InsertBytes(fb.Size()-int64(len(testdata)), testdata)
		default:
			fb.InsertBytes(fb.Size(), testdata)
		}
	}
}

func TestNewFileBuf(t *testing.T) {
	testfile, _ := os.CreateTemp("", "TESTFILE")
	defer os.Remove(testfile.Name())
	b, err := NewFileBuffer(testfile.Name())
	if err != nil {
		t.Fatalf("NewFileBuffer(%s): %v)", testfile.Name(), err)
	}
	if b == nil {
		t.Fatalf(`NewFileBuffer(%s) buf is 'nil'`, testfile.Name())
	}

	shittyAppend2Buf(b, testdata)

	testfile2, err := os.CreateTemp("", "TESTFILE")
	if err != nil {
		t.Fatalf("Couldn't open %s: %v", testfile2.Name(), err)
	}
	defer os.Remove(testfile2.Name())

	err = b.Dump(testfile2)
	/*n, err := io.Copy(f, b)
	if n != int64(len(testdata)) {
		t.Fatalf("Only copied %d, but expected %d", n, len(testdata))
	}
	*/
	if err != nil {
		t.Fatalf("Couldn't write %s: %v", testfile2.Name(), err)
	}

	if !compareBuf2File(b, testfile2) {
		t.Fatal("testfile != testfile2.Name()")
	}
	if !compareBuf2Bytes(b, testdata) {
		t.Fatal("testfile != testdata")
	}
}

func TestNewMemBuf(t *testing.T) {
	b := NewMemBuffer([]byte{})
	createTestData(b)

	testbuffer_size := int64(TESTDATA_REPEAT * len(testdata))
	if b.Size() != testbuffer_size {
		t.Fatalf("testMemBuf: testdata is wrong size (%d), should be (TESTDATA_REPEAT * %d = %d)", b.Size(), len(testdata), testbuffer_size)
	}

	testfile, err := os.CreateTemp("", "TESTFILE")
	if err != nil {
		t.Fatalf("Couldn't open %s: %v", testfile.Name(), err)
	}
	defer os.Remove(testfile.Name())

	err = b.Dump(testfile)
	if err != nil {
		t.Fatalf("Couldn't write %s: %v", testfile.Name(), err)
	}

	if !compareBuf2File(b, testfile) {
		t.Fatal("testMemBuf: buffer != testfile")
	}
}

func TestCut(t *testing.T) {
	b := NewMemBuffer([]byte{})
	createTestData(b)
	for i := TESTDATA_REPEAT - 1; i >= 0; i-- {
		cut := b.Cut(int64(i*len(testdata)), int64(len(helloworld)))
		if !compareBuf2Bytes(cut, helloworld) {
			t.Fatalf("Cut %d failed", i)
		}
	}
	if b.Size() != int64(TESTDATA_REPEAT*len(testdata)-TESTDATA_REPEAT*len(helloworld)) {
		t.Fatalf("Wrong size after cutting")
	}

	skipsz := int64(len(testdata) - len(helloworld) - len(testdata_line2))
	for i := int64(0); i < TESTDATA_REPEAT; i++ {
		cut := b.Cut(i*skipsz, int64(len(testdata_line2)))
		if !compareBuf2Bytes(cut, testdata_line2) {
			t.Fatalf("Second cut %d failed", i)
		}
	}
}

func TestPaste(t *testing.T) {
	b := NewMemBuffer(testdata)
	b2 := NewMemBuffer([]byte{})
	b3 := NewMemBuffer([]byte{})
	createTestData(b3)

	//createTestData by using Paste
	for i := 0; i < TESTDATA_REPEAT; i++ {
		switch i % 3 {
		case 0:
			b2.Paste(0, b)
		case 1:
			b2.Paste(b2.Size(), b)
		case 2:
			b2.Paste(int64(len(testdata)*(i%2)), b)
		}
	}

	if b2.Size() != b3.Size() {
		t.Fatalf("TestPaste: Wrong size")
	}

	//write b3 to file, compare to b2
	b3file, _ := os.CreateTemp("", "TESTFILE")
	defer os.Remove(b3file.Name())

	b3.Dump(b3file)
	if !compareBuf2File(b2, b3file) {
		t.Fatalf("TestPaste: b2 != b3file")
	}

	//write b2 to file, open b4 on file that b3 has written, compare them
	b2file, _ := os.CreateTemp("", "TESTFILE")
	defer os.Remove(b2file.Name())

	b2.Dump(b2file)
	b4, _ := NewFileBuffer(b3file.Name())
	if !compareBuf2File(b4, b2file) {
		t.Fatalf("TestPaste: newbuf(b2file) != b3file")
	}
}

func TestReadWriteSeek(t *testing.T) {
	b := NewMemBuffer([]byte{})
	createTestData(b)
	for i := 0; i < TESTDATA_REPEAT; i++ {
		// use a new buffer each iteration for fun
		buf := make([]byte, len(helloworld))
		i_off := int64(i * len(testdata))
		off, err := b.Seek(i_off, io.SeekStart)
		if err != nil {
			t.Fatalf("TestReadWriteSeek: seek1 failed: %v", err)
		}
		if off != i_off {
			t.Fatalf("TestReadWriteSeek: seek1 failed (unexpected offset)")
		}
		b.Read(buf)
		if !bytes.Equal(buf, helloworld) {
			t.Fatalf("TestReadWriteSeek: read1 failed")
		}
		off, err = b.Seek(-int64(len(helloworld)), io.SeekCurrent)
		if err != nil {
			t.Fatalf("TestReadWriteSeek: seek2 failed: %v", err)
		}
		if off != i_off {
			t.Fatalf("TestReadWriteSeek: seek2 failed (unexpected offset)")
		}
		b.Write(testdata)
		b.Write(helloworld)
	}
	newsz := TESTDATA_REPEAT*int64(len(testdata)) + int64(len(helloworld))
	if b.Size() != newsz {
		t.Fatalf("TestReadWriteSeek: Wrong size @ end (%d, should be %d)", b.Size(), newsz)
	}
	b.Remove(0, newsz-int64(len(helloworld)))
	n, _ := b.Seek(-int64(len(helloworld)), io.SeekEnd)
	if n != 0 {
		t.Fatalf("TestReadWriteSeek: unexpected size/seek : (%d/%d)", b.Size(), n)
	}
	buf := make([]byte, len(helloworld))
	b.Read(buf)
	if !bytes.Equal(buf, helloworld) {
		t.Fatalf("TestReadWriteSeek: final Read(..) failed")
	}
}

//return offset, size, cut
func randomCut(t *testing.T, b *FileBuffer) (int64, int64, *FileBuffer) {
	if b.Size() <= 0 {
		return 0, 0, NewMemBuffer([]byte{})
	}
	offset := rand.Int63n(b.Size())
	size := rand.Int63n(b.Size() - offset)
	//fmt.Println("Randomcut", offset, size)
	cut := b.Cut(offset, size)
	if cut.Size() != size {
		t.Fatalf("randomCut: cut is not the right size")
	}
	/* XXX TODO?
	seekoff, err := b.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatalf("randomCut: seek failed: %v", err)
	}
	if seekoff != offset {
		t.Fatalf("randomCut: cut is not at right offset")
	}
	*/
	return offset, size, cut
}

func TestCutCopyPaste(t *testing.T) {
	rand.Seed(time.Hour.Milliseconds())
	b := NewMemBuffer([]byte{})
	//don't touch btest to compare at the end
	btest := NewMemBuffer([]byte{})
	createTestData(btest)
	createTestData(b)

	testfile, err := os.CreateTemp("", "TESTFILE")
	if err != nil {
		t.Fatalf("Couldn't open %s: %v", testfile.Name(), err)
	}
	defer os.Remove(testfile.Name())
	err = btest.Dump(testfile)
	if err != nil {
		t.Fatalf("Couldn't write %s: %v", testfile.Name(), err)
	}

	if b.Size() != btest.Size() {
		t.Fatalf("TestCutCopyPaste: size didn't return to original size")
	}
	if !compareBuf2File(b, testfile) {
		t.Fatal("TestCutCopyPaste: before cutting n pasting: buffer != testfile")
	}

	for i := 0; i < TESTDATA_REPEAT/20; i++ {
		o1, _, c1 := randomCut(t, b)
		p1 := c1.Copy(0, c1.Size())
		o2, _, c2 := randomCut(t, c1)
		p2 := c2.Copy(0, c2.Size())
		o3, _, c3 := randomCut(t, c2)
		p3 := c3.Copy(0, c3.Size())

		c2.Paste(o3, p3)
		c1.Paste(o2, p2)
		b.Paste(o1, p1)
	}

	if !compareBuf2File(b, testfile) {
		t.Fatal("TestCutCopyPaste: after everything, buffer != testfile")
	}
}
