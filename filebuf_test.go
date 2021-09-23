package filebuf

/* TODO:
 * - these tests only test working basic functionality and don't try to trigger errors
 *   should be about half/half imo
 * - more tests with file-backed filebuf's
 */

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"testing"
	"time"

	//other rope implementations
	//R1 "github.com/vinzmay/go-rope"
	R1 "github.com/vinzmay/go-rope"
	//R2 "github.com/fvbommel/util/rope"
	R2 "github.com/fvbommel/util/rope"
	//R3 "github.com/eaburns/T/rope"
	R3 "github.com/eaburns/T/rope"
	//R4 "github.com/zyedidia/rope"
	R4 "github.com/zyedidia/rope"
)

const TESTDATA_REPEAT = 5000

var helloworld = []byte("Hello World!\n")
var testdata_line2 = []byte("this is some testdata.\n")
var testdata = []byte(`Hello World!
this is some testdata.
this is the third line.

.



...

.

.
`)

//compare a filebuf to another file
//XXX this slurps entire file; only use smallish files here
func compareBuf2File(b *Buffer, f io.ReadSeeker) bool {
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
func compareBuf2Bytes(b *Buffer, s []byte) bool {
	buf := make([]byte, 1)
	b.Seek(0, io.SeekStart)
	for i := 0; i < len(s); i++ {
		b.Read(buf)
		if buf[0] != s[i] {
			return false
		}
	}
	return true
}

//create lots of new nodes by adding b byte-per-byte, backwards, to the end of fb
func shittyAppend(fb *Buffer, b []byte) error {
	for i := 1; i <= len(b); i++ {
		fb.Insert1(0, b[len(b)-i])
		if fb == nil {
			return nil
		}
	}
	return nil
}

//add a small bunch of data in a sort of randomized way to fb
//the final buffer should hold testdata, repeated TESTDATA_REPEAT times
func createTestData(fb *Buffer) {
	for n := 0; n < TESTDATA_REPEAT; n++ {
		switch n % 5 {
		case 0:
			fb.Seek(0, io.SeekEnd)
			for i := 0; i < len(testdata); i++ {
				fb.Write(testdata[i : i+1])
			}
		case 1:
			fb.Insert(0, testdata)
		case 2:
			shittyAppend(fb, testdata)
		case 3:
			fb.Insert(fb.Size()-int64(len(testdata)), testdata)
		default:
			fb.Insert(fb.Size(), testdata)
		}
	}
}
func createTestData_(fb *Buffer) {
	for n := 0; n < TESTDATA_REPEAT; n++ {
		fb.Insert(0, benchText)
	}
}

func TestNewFileBuf(t *testing.T) {
	testfile, _ := os.CreateTemp("", "TESTFILE")
	defer os.Remove(testfile.Name())
	b, err := OpenFile(testfile.Name())
	if err != nil {
		t.Fatalf("OpenFile(%s): %v)", testfile.Name(), err)
	}
	if b == nil {
		t.Fatalf(`OpenFile(%s) buf is 'nil'`, testfile.Name())
	}

	shittyAppend(b, testdata)

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

func TestNewMem(t *testing.T) {
	b := NewEmpty()
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
	b := NewEmpty()
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
	b := NewMem(testdata)
	b2 := NewEmpty()
	b3 := NewEmpty()
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
	b4, _ := OpenFile(b3file.Name())
	if !compareBuf2File(b4, b2file) {
		t.Fatalf("TestPaste: newbuf(b2file) != b3file")
	}
}

func TestReadWriteSeek(t *testing.T) {
	b := NewEmpty()
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
func randomCut(t *testing.T, b *Buffer) (int64, int64, *Buffer) {
	if b.Size() <= 0 {
		return 0, 0, NewEmpty()
	}
	offset := rand.Int63n(b.Size())
	size := rand.Int63n(b.Size() - offset)
	//fmt.Println("Randomcut", offset, size)
	cut := b.Cut(offset, size)
	if cut.Size() != size {
		t.Fatalf("randomCut: cut is not the right size")
	}
	return offset, size, cut
}

func TestCutCopyPaste(t *testing.T) {
	rand.Seed(time.Hour.Milliseconds())
	b := NewEmpty()
	//don't touch btest to compare at the end
	btest := NewEmpty()
	createTestData_(btest)
	createTestData_(b)

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

	for i := 0; i < TESTDATA_REPEAT/10; i++ {
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
	//btest.Stats("Normal testdata")
	//b.Stats("Chopped up testdata (lot of cuts & pastes)")

	if !compareBuf2File(b, testfile) {
		t.Fatal("TestCutCopyPaste: after everything, buffer != testfile")
	}
}

func TestFileBufVsOtherImplementation(t *testing.T) {
	b := NewEmpty()
	b2 := R2.New("")
	for i := 0; i < TESTDATA_REPEAT; i++ {
		if b.Size() != b2.Len() {
			t.Fatal("size doesn't match other implementation")
		}
		w := benchWord()
		off := benchInt64(b.Size())
		b.Insert(off, w)
		b2 = R2Insert(b2, off, string(w))
	}
	b2Bytes := []byte(b2.String())
	if !compareBuf2Bytes(b, b2Bytes) {
		t.Fatal("buffer contents doesn't match other implementation after inserts")
	}

	//now write our memory buffer to a file, open that and test some more
	f, err := os.CreateTemp("", "")
	defer os.Remove(f.Name())
	if err != nil {
		t.Fatal("Couldn't create tempfile")
	}
	b.Dump(f)
	tmpfileName := f.Name()
	f.Close()
	b, err = OpenFile(tmpfileName)
	if err != nil {
		t.Fatalf("Couldn't open %s!", tmpfileName)
	}
	for i := 0; i < TESTDATA_REPEAT; i++ {
		if b.Size() != b2.Len() {
			t.Fatal("size doesn't match other implementation")
		}
		w := benchWord()
		off := benchInt64(b.Size())
		b.Insert(off, w)
		b2 = R2Insert(b2, off, string(w))
	}
	b2Bytes = []byte(b2.String())
	if !compareBuf2Bytes(b, b2Bytes) {
		t.Fatal("buffer contents doesn't match other implementation after inserts")
	}

	for i := 0; i < TESTDATA_REPEAT; i++ {
		if b.Size() != b2.Len() {
			t.Fatal("size doesn't match other implementation (deleting)")
		}
		off := benchInt64(b.Size())
		size := benchInt64(b.Size() - off)

		b.Remove(off, size)
		b2 = b2.Slice(0, off).Append(b2.Slice(off+size, b2.Len()))
	}
	b2Bytes = []byte(b2.String())
	if !compareBuf2Bytes(b, b2Bytes) {
		t.Fatal("buffer contents doesn't match other implementation after deletions")
	}

}
func TestMemBufVsOtherImplementation(t *testing.T) {
	//R2 seems solid
	b := NewEmpty()
	b2 := R2.New("")
	for i := 0; i < TESTDATA_REPEAT; i++ {
		if b.Size() != b2.Len() {
			t.Fatal("size doesn't match other implementation")
		}
		w := benchWord()
		off := benchInt64(b.Size())
		b.Insert(off, w)
		b2 = R2Insert(b2, off, string(w))
	}
	b2Bytes := []byte(b2.String())
	if !compareBuf2Bytes(b, b2Bytes) {
		t.Fatal("buffer contents doesn't match other implementation after inserts")
	}

	for i := 0; i < TESTDATA_REPEAT; i++ {
		if b.Size() != b2.Len() {
			t.Fatal("size doesn't match other implementation (deleting)")
		}
		off := benchInt64(b.Size())
		size := benchInt64(b.Size() - off)

		b.Remove(off, size)
		b2 = b2.Slice(0, off).Append(b2.Slice(off+size, b2.Len()))
	}
	b2Bytes = []byte(b2.String())
	if !compareBuf2Bytes(b, b2Bytes) {
		t.Fatal("buffer contents doesn't match other implementation after deletions")
	}

}

/* BENCHMARKING functions */

//testing variables
var benchmark_random_seed = rand.NewSource(time.Now().Unix())
var bench_rnd = rand.New(benchmark_random_seed)
var benchWords = bytes.Fields(benchText)

//call this at the start of each benchmark function
func startBench(b *testing.B) {
	bench_rnd = rand.New(benchmark_random_seed)
	b.ResetTimer()
}

func benchWord() []byte {
	return benchWords[bench_rnd.Intn(len(benchWords))]
}

func benchInt64(max int64) int64 {
	if max == 0 {
		return 0
	}
	return bench_rnd.Int63n(max)
}

func benchInt(max int) int {
	if max == 0 {
		return 0
	}
	return bench_rnd.Intn(max)
}

//R0 is my own implementation ("github.com/snhmibby/filebuf")
func BenchmarkInsertR0(b *testing.B) {
	startBench(b)
	buf := NewEmpty()
	for i := 0; i < b.N; i++ {
		w := benchWord()
		offs := benchInt64(buf.Size())
		buf.Insert(offs, w)
	}
}

func BenchmarkCopyR0(b *testing.B) {
	buf := NewEmpty()
	//create some data
	for i := 0; i < TESTDATA_REPEAT; i++ {
		off := benchInt64(buf.Size())
		buf.Insert(off, benchWord())
	}

	startBench(b)
	for i := 0; i < b.N; i++ {
		off := benchInt64(buf.Size())
		sz := benchInt64((buf.Size() - off) / 40)
		_ = buf.Copy(off, sz)
	}
}

func BenchmarkInsertCutPasteR0(b *testing.B) {
	startBench(b)
	buf := NewEmpty()
	paste := NewMem(benchWord())
	for i := 0; i < b.N; i++ {
		offs := benchInt64(buf.Size())
		switch i % 5 {
		case 0, 1, 2: //Just insert
			buf.Insert(offs, benchWord())
		case 3:
			buf.Paste(offs, paste)
		case 4:
			buf.Cut(offs, benchInt64(buf.Size()-offs))
		}
	}
}

//benchmark R1 ("github.com/vinzmay/go-rope")
func BenchmarkInsertR1(b *testing.B) {
	startBench(b)
	buf := R1.New("")
	for i := 0; i < b.N; i++ {
		w := string(benchWord())
		offs := benchInt(buf.Len())
		buf = buf.Insert(offs, w)
	}
}

func BenchmarkCopyR1(b *testing.B) {
	buf := R1.New("")
	//create some data
	for i := 0; i < TESTDATA_REPEAT; i++ {
		off := benchInt(buf.Len())
		buf.Insert(off, string(benchWord()))
	}

	startBench(b)
	for i := 0; i < b.N; i++ {
		off := benchInt(buf.Len())
		//same bug as below
		if off == 0 {
			continue
		}
		sz := benchInt(buf.Len() - off)
		_ = buf.Substr(off, sz)
	}
}

func BenchmarkInsertCutPasteR1(b *testing.B) {
	startBench(b)
	buf := R1.New("")
	paste := R1.New(string(benchWord()))
	for i := 0; i < b.N; i++ {
		offs := benchInt(buf.Len())
		switch i % 5 {
		case 0, 1, 2: //Just insert
			buf = buf.Insert(offs, string(benchWord()))
		case 3:
			//buf = buf.Paste(offs, paste)
			l, r := buf.Split(offs)
			buf = l.Concat(paste).Concat(r)
		case 4:
			//buf.Cut(offs, benchInt64(buf.Size()-offs))
			//XXX seems to be a bug in R1, offset cannot be 0??
			if offs == 0 {
				if buf.Len() <= 0 {
					continue
				}
				offs = 1
			}
			size := benchInt(buf.Len() - offs)
			buf = buf.Delete(offs, size)
		}
	}
}

//benchmark R2 ("github.com/fvbommel/util/rope")
func R2Insert(rope R2.Rope, offs int64, w string) R2.Rope {
	l := rope.Slice(0, offs)
	r := rope.Slice(offs, rope.Len())
	p := R2.New(w)
	return l.Append(p).Append(r)
}

func BenchmarkInsertR2(b *testing.B) {
	startBench(b)
	buf := R2.New("")
	for i := 0; i < b.N; i++ {
		w := string(benchWord())
		offs := benchInt64(buf.Len())
		buf = R2Insert(buf, offs, w)
	}
}

func BenchmarkCopyR2(b *testing.B) {
	buf := R2.New("")
	//create some data
	for i := 0; i < TESTDATA_REPEAT; i++ {
		off := benchInt64(buf.Len())
		buf = R2Insert(buf, off, string(benchWord()))
	}

	startBench(b)
	for i := 0; i < b.N; i++ {
		off := benchInt64(buf.Len())
		sz := benchInt64(buf.Len() - off)
		_ = buf.Slice(off, off+sz)
	}
}

func BenchmarkInsertCutPasteR2(b *testing.B) {
	startBench(b)
	buf := R2.New("")
	for i := 0; i < b.N; i++ {
		offs := benchInt64(buf.Len())
		switch i % 5 {
		case 0, 1, 2, 3: //Just insert
			buf = R2Insert(buf, offs, string(benchWord()))
		case 4:
			size := benchInt64(buf.Len() - offs)
			buf = buf.Slice(0, offs).Append(buf.Slice(offs+size, buf.Len()))
		}
	}
}

//benchmark R3 ("github.com/eaburns/T/rope)
func R3Insert(rope R3.Rope, offs int64, w string) R3.Rope {
	p := R3.New(w)
	return R3.Insert(rope, offs, p)
}

func BenchmarkInsertR3(b *testing.B) {
	startBench(b)
	buf := R3.New("")
	for i := 0; i < b.N; i++ {
		w := string(benchWord())
		offs := benchInt64(buf.Len())
		buf = R3Insert(buf, offs, w)
	}
}

func BenchmarkInsertCutPasteR3(b *testing.B) {
	startBench(b)
	buf := R3.New("")
	for i := 0; i < b.N; i++ {
		offs := benchInt64(buf.Len())
		switch i % 5 {
		case 0, 1, 2, 3: //Just insert
			buf = R3Insert(buf, offs, string(benchWord()))
		case 4:
			size := benchInt64(buf.Len() - offs)
			buf = R3.Delete(buf, offs, size)
		}
	}
}

//benchmark R4 ("github.com/eaburns/T/rope)
func BenchmarkInsertR4(b *testing.B) {
	startBench(b)
	buf := R4.New([]byte(""))
	for i := 0; i < b.N; i++ {
		w := benchWord()
		offs := benchInt(buf.Len())
		buf.Insert(offs, w)
	}
}

func BenchmarkInsertCutPasteR4(b *testing.B) {
	startBench(b)
	buf := R4.New([]byte(""))
	for i := 0; i < b.N; i++ {
		offs := benchInt(buf.Len())
		switch i % 5 {
		case 0, 1, 2, 3: //Just insert
			buf.Insert(offs, benchWord())
		case 4:
			size := benchInt(buf.Len() - offs)
			buf.Remove(offs, size)
		}
	}
}

var benchText = []byte(`
Lorem Ipsum
"Neque porro quisquam est qui dolorem ipsum quia dolor sit amet, consectetur, adipisci velit..."
"There is no one who loves pain itself, who seeks after it and wants to have it, simply because it is pain..."
What is Lorem Ipsum?

Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.
Why do we use it?

It is a long established fact that a reader will be distracted by the readable content of a page when looking at its layout. The point of using Lorem Ipsum is that it has a more-or-less normal distribution of letters, as opposed to using 'Content here, content here', making it look like readable English. Many desktop publishing packages and web page editors now use Lorem Ipsum as their default model text, and a search for 'lorem ipsum' will uncover many web sites still in their infancy. Various versions have evolved over the years, sometimes by accident, sometimes on purpose (injected humour and the like).
Where does it come from?

Contrary to popular belief, Lorem Ipsum is not simply random text. It has roots in a piece of classical Latin literature from 45 BC, making it over 2000 years old. Richard McClintock, a Latin professor at Hampden-Sydney College in Virginia, looked up one of the more obscure Latin words, consectetur, from a Lorem Ipsum passage, and going through the cites of the word in classical literature, discovered the undoubtable source. Lorem Ipsum comes from sections 1.10.32 and 1.10.33 of "de Finibus Bonorum et Malorum" (The Extremes of Good and Evil) by Cicero, written in 45 BC. This book is a treatise on the theory of ethics, very popular during the Renaissance. The first line of Lorem Ipsum, "Lorem ipsum dolor sit amet..", comes from a line in section 1.10.32.

The standard chunk of Lorem Ipsum used since the 1500s is reproduced below for those interested. Sections 1.10.32 and 1.10.33 from "de Finibus Bonorum et Malorum" by Cicero are also reproduced in their exact original form, accompanied by English versions from the 1914 translation by H. Rackham.
Where can I get some?

There are many variations of passages of Lorem Ipsum available, but the majority have suffered alteration in some form, by injected humour, or randomised words which don't look even slightly believable. If you are going to use a passage of Lorem Ipsum, you need to be sure there isn't anything embarrassing hidden in the middle of text. All the Lorem Ipsum generators on the Internet tend to repeat predefined chunks as necessary, making this the first true generator on the Internet. It uses a dictionary of over 200 Latin words, combined with a handful of model sentence structures, to generate Lorem Ipsum which looks reasonable. The generated Lorem Ipsum is therefore always free from repetition, injected humour, or non-characteristic words etc.
	
	paragraphs
	words
	bytes
	lists
		Start with 'Lorem
ipsum dolor sit amet...'
	
Translations: Can you help translate this site into a foreign language ? Please email us with details if you can help.
There are now a set of mock banners available here in three colours and in a range of standard banner sizes:
BannersBannersBanners
Donate: If you use this site regularly and would like to help keep the site on the Internet, please consider donating a small sum to help pay for the hosting and bandwidth bill. There is no minimum donation, any sum is appreciated - click here to donate using PayPal. Thank you for your support.
Donate Bitcoin: 16UQLq1HZ3CNwhvgrarV6pMoA2CDjb4tyF
NodeJS Python Interface GTK Lipsum Rails .NET Groovy
The standard Lorem Ipsum passage, used since the 1500s

"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."
Section 1.10.32 of "de Finibus Bonorum et Malorum", written by Cicero in 45 BC

"Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam, eaque ipsa quae ab illo inventore veritatis et quasi architecto beatae vitae dicta sunt explicabo. Nemo enim ipsam voluptatem quia voluptas sit aspernatur aut odit aut fugit, sed quia consequuntur magni dolores eos qui ratione voluptatem sequi nesciunt. Neque porro quisquam est, qui dolorem ipsum quia dolor sit amet, consectetur, adipisci velit, sed quia non numquam eius modi tempora incidunt ut labore et dolore magnam aliquam quaerat voluptatem. Ut enim ad minima veniam, quis nostrum exercitationem ullam corporis suscipit laboriosam, nisi ut aliquid ex ea commodi consequatur? Quis autem vel eum iure reprehenderit qui in ea voluptate velit esse quam nihil molestiae consequatur, vel illum qui dolorem eum fugiat quo voluptas nulla pariatur?"
1914 translation by H. Rackham

"But I must explain to you how all this mistaken idea of denouncing pleasure and praising pain was born and I will give you a complete account of the system, and expound the actual teachings of the great explorer of the truth, the master-builder of human happiness. No one rejects, dislikes, or avoids pleasure itself, because it is pleasure, but because those who do not know how to pursue pleasure rationally encounter consequences that are extremely painful. Nor again is there anyone who loves or pursues or desires to obtain pain of itself, because it is pain, but because occasionally circumstances occur in which toil and pain can procure him some great pleasure. To take a trivial example, which of us ever undertakes laborious physical exercise, except to obtain some advantage from it? But who has any right to find fault with a man who chooses to enjoy a pleasure that has no annoying consequences, or one who avoids a pain that produces no resultant pleasure?"
Section 1.10.33 of "de Finibus Bonorum et Malorum", written by Cicero in 45 BC

"At vero eos et accusamus et iusto odio dignissimos ducimus qui blanditiis praesentium voluptatum deleniti atque corrupti quos dolores et quas molestias excepturi sint occaecati cupiditate non provident, similique sunt in culpa qui officia deserunt mollitia animi, id est laborum et dolorum fuga. Et harum quidem rerum facilis est et expedita distinctio. Nam libero tempore, cum soluta nobis est eligendi optio cumque nihil impedit quo minus id quod maxime placeat facere possimus, omnis voluptas assumenda est, omnis dolor repellendus. Temporibus autem quibusdam et aut officiis debitis aut rerum necessitatibus saepe eveniet ut et voluptates repudiandae sint et molestiae non recusandae. Itaque earum rerum hic tenetur a sapiente delectus, ut aut reiciendis voluptatibus maiores alias consequatur aut perferendis doloribus asperiores repellat."
1914 translation by H. Rackham

"On the other hand, we denounce with righteous indignation and dislike men who are so beguiled and demoralized by the charms of pleasure of the moment, so blinded by desire, that they cannot foresee the pain and trouble that are bound to ensue; and equal blame belongs to those who fail in their duty through weakness of will, which is the same as saying through shrinking from toil and pain. These cases are perfectly simple and easy to distinguish. In a free hour, when our power of choice is untrammelled and when nothing prevents our being able to do what we like best, every pleasure is to be welcomed and every pain avoided. But in certain circumstances and owing to the claims of duty or the obligations of business it will frequently occur that pleasures have to be repudiated and annoyances accepted. The wise man therefore always holds in these matters to this principle of selection: he rejects pleasures to secure other greater pleasures, or else he endures pains to avoid worse pains."
help@lipsum.com
Privacy Policy
`)
