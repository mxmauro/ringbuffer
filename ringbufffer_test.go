package ringbuffer_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/mxmauro/ringbuffer"
)

// -----------------------------------------------------------------------------

type testRingBuffer struct {
	t  *testing.T
	rb *ringbuffer.RingBuffer
}

var testData = []byte("Hello")

// -----------------------------------------------------------------------------

func TestRingBuffer(t *testing.T) {
	trb := &testRingBuffer{
		t:  t,
		rb: ringbuffer.New(32),
	}

	t.Run("single write and read", func(_ *testing.T) {
		trb.writeHello()
		trb.readHello()
		trb.checkLen(0)

		trb.readEOF()
	})

	t.Run("multiple writes and reads", func(_ *testing.T) {
		trb.writeHello()
		trb.writeHello()
		trb.checkLen(10)
		trb.readHello()
		trb.checkLen(5)
		trb.writeHello()
		trb.checkLen(10)
	})

	t.Run("buffer expansion", func(_ *testing.T) {
		for i := 1; i <= 100; i++ {
			trb.writeHello()
		}
		trb.checkLen(510)

		for i := 1; i <= 20; i++ {
			trb.readHello()
		}
		trb.checkLen(410)
	})

	t.Run("consume pending", func(_ *testing.T) {
		for i := 1; i <= 82; i++ {
			trb.readHello()
		}
		trb.readEOF()
	})
}

func (trb *testRingBuffer) readHello() {
	var buf [5]byte

	n, err := trb.rb.Read(buf[:])
	if err != nil {
		trb.t.Fatal(err)
	}
	if n != len(buf) {
		trb.t.Fatal("read data length mismatch")
	}
	if bytes.Compare(buf[:], testData) != 0 {
		trb.t.Fatal("invalid data read")
	}
}

func (trb *testRingBuffer) readEOF() {
	var buf [5]byte

	_, err := trb.rb.Read(buf[:])
	if err != io.EOF {
		trb.t.Fatal("expected EOF")
	}
}

func (trb *testRingBuffer) writeHello() {
	n, err := trb.rb.Write(testData)
	if err != nil {
		trb.t.Fatal(err)
	}
	if n != len(testData) {
		trb.t.Fatal("written data length mismatch")
	}
}

func (trb *testRingBuffer) checkLen(expected int) {
	if trb.rb.Len() != expected {
		trb.t.Fatal("unexpected buffer length")
	}
}
