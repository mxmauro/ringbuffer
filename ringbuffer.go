package ringbuffer

import (
	"errors"
	"io"
	"sync"
)

// -----------------------------------------------------------------------------

// RingBuffer represents a thread-safe circular buffer.
type RingBuffer struct {
	mtx      sync.Mutex
	buf      []byte
	growSize int
	readPos  int // Holds the read-position in the buffer.
	written  int // Holds the number of bytes written to the buffer.
}

// -----------------------------------------------------------------------------

// New returns a new circular buffer with an initial size.
// If the buffer needs to be expanded, it will be expanded to the next
// power of two greater than the requested size.
func New(growSize int) *RingBuffer {
	// Create and initialize the ring buffer.
	r := &RingBuffer{}
	r.Initialize(growSize)

	// Done
	return r
}

// Initialize initializes a circular buffer with an initial size.
// If the buffer needs to be expanded, it will be expanded to the next
// power of two greater than the requested size.
func (r *RingBuffer) Initialize(growSize int) {
	if growSize <= 15 {
		growSize = 16
	} else if growSize > 1048576 {
		growSize = 1048576
	} else {
		growSize -= 1
		growSize |= growSize >> 1
		growSize |= growSize >> 2
		growSize |= growSize >> 4
		growSize |= growSize >> 8
		growSize |= growSize >> 16
		growSize += 1
	}

	// Initialize the ring buffer.
	r.buf = make([]byte, growSize)
	r.growSize = growSize
}

// Peek reads up to len(p) bytes from the buffer without advancing the read-position.
// It returns the number of bytes read and any error encountered.
// At the end of the buffer, Peek returns 0, io.EOF.
func (r *RingBuffer) Peek(p []byte) (n int, err error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	// Read from the buffer.
	return r.peek(p)
}

// Read reads up to len(p) bytes from the buffer and stores them in p.
// It returns the number of bytes read and any error encountered.
// At the end of the buffer, Read returns 0, io.EOF.
func (r *RingBuffer) Read(p []byte) (n int, err error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	// Read from the buffer.
	n, err = r.peek(p)
	if err == nil {
		// Advance the read-position.
		r.advanceReadPos(n)
	}
	return
}

// Write writes len(p) bytes from p to the buffer.
// It returns the number of bytes written and an error, if any.
// Write returns a non-nil error when n != len(p).
func (r *RingBuffer) Write(p []byte) (n int, err error) {
	n = len(p)
	if n == 0 {
		return 0, nil
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()

	// Ensure there is enough space to hold the new data.
	err = r.ensureCapacity(n)
	if err != nil {
		n = 0
		return
	}

	// Get the writable portion of the buffer.
	ofs1, len1, len2 := r.writeInfo()
	if n <= len1 {
		copy(r.buf[ofs1:ofs1+n], p)
	} else {
		copy(r.buf[ofs1:], p[:len1])
		copy(r.buf[:len2], p[len1:])
	}

	// Advance the write-position.
	r.advanceWritePos(n)

	// Done
	return
}

// Find returns the index of the first occurrence of b in the unread portion of the buffer,
// or -1 if b is not present in the buffer.
func (r *RingBuffer) Find(b byte) int {
	foundIdx := -1
	r.Scan(func(elem byte, idx int) bool {
		if elem == b {
			foundIdx = idx
			return true
		}
		return false
	})
	return foundIdx
}

// FindBytes returns the index of the first occurrence of the slice b in the unread portion of the buffer,
// or -1 if b is not present in the buffer.
func (r *RingBuffer) FindBytes(b []byte) int {
	if len(b) == 0 {
		return -1
	}

	foundIdx := -1
	currentOfs := 0
	potentialStartIdx := -1
	r.Scan(func(elem byte, idx int) bool {
		if elem == b[currentOfs] {
			if currentOfs == 0 {
				potentialStartIdx = idx
			}
			currentOfs += 1
			if currentOfs == len(b) {
				foundIdx = potentialStartIdx
				return true
			}
		} else {
			currentOfs = 0
		}
		return false
	})
	return foundIdx
}

// Scan calls fn for each byte in the unread portion of the buffer.
// If the callback returns false, Scan stops the iteration.
func (r *RingBuffer) Scan(fn func(elem byte, idx int) bool) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	ofs1, len1, len2 := r.readInfo()

	for idx := 0; idx < len1; idx++ {
		stop := fn(r.buf[ofs1+idx], idx)
		if stop {
			return
		}
	}
	for idx := 0; idx < len2; idx++ {
		stop := fn(r.buf[idx], len1+idx)
		if stop {
			return
		}
	}
}

// Len returns the number of bytes of the unread portion of the buffer.
func (r *RingBuffer) Len() int {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	return r.written
}

func (r *RingBuffer) readInfo() (ofs1 int, len1 int, len2 int) {
	ofs1 = r.readPos
	if r.readPos <= len(r.buf)-r.written {
		len1 = r.written
	} else {
		len1 = len(r.buf) - r.readPos
		len2 = r.written - (len(r.buf) - r.readPos)
	}
	return
}

func (r *RingBuffer) writeInfo() (ofs1 int, len1 int, len2 int) {
	if r.written == len(r.buf) {
		return
	}
	if r.readPos < len(r.buf)-r.written {
		end := r.readPos + r.written
		ofs1 = end
		len1 = len(r.buf) - end
		if r.readPos > 0 {
			len2 = r.readPos
		}
	} else {
		ofs1 = r.readPos - (len(r.buf) - r.written)
		len1 = len(r.buf) - r.written
	}
	return
}

func (r *RingBuffer) ensureCapacity(n int) error {
	if n > len(r.buf)-r.written {
		required := r.written + n
		if required < n {
			return errors.New("buffer overflow")
		}
		rem := required % r.growSize
		newSize := required + (r.growSize - rem)
		r.growBuffer(newSize)
	}
	return nil
}

func (r *RingBuffer) growBuffer(newSize int) {
	if newSize > len(r.buf) {
		newBuf := make([]byte, newSize)

		if r.readPos+r.written <= len(r.buf) {
			copy(newBuf, r.buf[r.readPos:r.readPos+r.written])
		} else {
			temp := len(r.buf) - r.readPos
			copy(newBuf, r.buf[r.readPos:])
			copy(newBuf[temp:], r.buf[:r.written-temp])
		}

		r.buf = newBuf
		r.readPos = 0
	}
}

func (r *RingBuffer) advanceReadPos(n int) {
	if r.readPos < len(r.buf)-n {
		r.readPos += n
	} else {
		r.readPos -= len(r.buf) - n
	}
	r.written -= n
}

func (r *RingBuffer) advanceWritePos(n int) {
	r.written += n
}

func (r *RingBuffer) peek(buf []byte) (int, error) {
	n := len(buf)
	if n == 0 {
		return 0, nil
	}

	ofs1, len1, len2 := r.readInfo()

	if len1 == 0 && len2 == 0 {
		return 0, io.EOF // Nothing to read.
	}

	if n <= len1 {
		copy(buf, r.buf[ofs1:ofs1+n])
	} else {
		if n > len1+len2 {
			n = len1 + len2
		}
		copy(buf, r.buf[ofs1:])
		copy(buf[len1:], r.buf[:len2])
	}

	return n, nil
}
