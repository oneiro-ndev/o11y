package honeycomb

import (
	"encoding/hex"
	"fmt"
	"io"
	"sync"
)

// CircularBuffer is an io.ReadWriter that implements a...you guessed it,
// circular buffer of bytes. You set the buffer size, and you can write to it
// until it's full, and then you can read from it until it's empty again. It's
// safe to use in concurrent operations.
//
// Its purpose is to provide a safe way to pass an io.Writer to a separate
// process for its stdout or stderr, and to have a local goroutine reading and
// parsing the stream of data in real time.
//
// The C member is a chan struct{} that receives a new value for every operation
// that changes the buffer's length to a nonzero value (every successful
// nonempty Write and any partial Consume). In other words, listening to this
// channel will tell you whenever there is something in the buffer to read.
// However, redundant notifications will be dropped on the floor. So if you call
// write 3 times but don't read from the channel until later, you'll only see a
// single event. Calling Close() also closes this channel.
//
// The Peek operation is a read without advancing the read pointer. This can be
// useful in parsing situations.
//
// Calling Close() prevents further writes, and after the entire input has been
// consumed, returns EOF for further read operations.
type CircularBuffer struct {
	C chan struct{}

	mutex  sync.Mutex
	buf    []byte
	len    int
	index  int
	closed bool
}

var _ io.ReadWriteCloser = (*CircularBuffer)(nil)

func (c *CircularBuffer) String() string {
	return fmt.Sprintf("%s index: %d   len: %d", hex.EncodeToString(c.buf), c.index, c.len)
}

// NewCircularBuffer builds a CircularBuffer of the specified capacity.
func NewCircularBuffer(capacity int) *CircularBuffer {
	return &CircularBuffer{
		C:     make(chan struct{}, 1),
		buf:   make([]byte, capacity),
		len:   0,
		index: 0,
	}
}

// Write implements io.Writer for CircularBuffer. Note that if all of p cannot be written to the
// buffer as it stands, the buffer's capacity is grown. This call will return io.EOF if
// Close() has been called; otherwise it will only error if the buffer cannot be expanded.
func (c *CircularBuffer) Write(p []byte) (int, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.closed {
		return 0, io.EOF
	}
	if len(p) > len(c.buf)-c.len {
		_, err := c.resize(len(p) + c.len)
		if err != nil {
			return 0, err
		}
	}

	startWritingAt := (c.index + c.len) % len(c.buf)
	leftBeforeEnd := len(c.buf) - startWritingAt
	n := 0
	if len(p) < leftBeforeEnd {
		// it all fits in before it's time to wrap
		n = copy(c.buf[startWritingAt:], p)
		c.addLen(len(p))
	} else {
		// it didn't all fit in before we have to wrap
		n = copy(c.buf[startWritingAt:], p[:leftBeforeEnd])
		n += copy(c.buf[:startWritingAt], p[leftBeforeEnd:])
		c.addLen(len(p))
	}
	return n, nil
}

// Read implements io.Reader for CircularBuffer. It is the equivalent of calling
// Peek() followed by Consume(), but is locked for the entire time.
func (c *CircularBuffer) Read(p []byte) (int, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	n, err := c.peek(p)
	if err != nil {
		return n, err
	}
	return c.consume(n), nil
}

// Peek retrieves the leading bytes from the buffer but does not move the index
// pointer. It returns as many bytes as the maximum of the length of p or the current
// length of the buffer.
func (c *CircularBuffer) Peek(p []byte) (int, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.peek(p)
}

// Consume advances the index pointer by n bytes, or by the current length of the input,
// whichever is shorter. It returns the number of bytes actually advanced.
func (c *CircularBuffer) Consume(n int) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.consume(n)
}

// Capacity returns the capacity of the buffer; this is the value set
// at initialization.
func (c *CircularBuffer) Capacity() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return len(c.buf)
}

// Len returns the current length of the buffer.
func (c *CircularBuffer) Len() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.len
}

// Close implements io.Closer for CircularBuffer
func (c *CircularBuffer) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.closed = true
	close(c.C)
	return nil
}

// Private API below here
// Note to maintainers:
// all public methods (except Listen) must use a mutex, and no private ones should.

func (c *CircularBuffer) addLen(n int) {
	c.len += n
	// when we set the length, if it's nonzero, send the value on
	// the channel, but don't block if the channel is already full
	if c.len != 0 && !c.closed {
		select {
		case c.C <- struct{}{}:
		default:
		}
	}
}

// consume advances the index pointer by n bytes, or by the current length of the input,
// whichever is shorter. It returns the number of bytes actually advanced.
// It does not lock.
func (c *CircularBuffer) consume(n int) int {
	if n > c.len {
		n = c.len
	}
	c.index = (c.index + n) % len(c.buf)
	c.addLen(-n)
	return n
}

// peek retrieves the contents of the circular buffer into
// []byte; it does not lock or change any pointers.
func (c *CircularBuffer) peek(p []byte) (int, error) {
	if c.len == 0 && c.closed {
		return 0, io.EOF
	}
	l := len(p)
	if l > c.len {
		l = c.len
	}
	n := 0
	leftBeforeEnd := len(c.buf) - c.index
	if l < leftBeforeEnd {
		n = copy(p, c.buf[c.index:c.index+l])
	} else {
		n = copy(p, c.buf[c.index:])
		n += copy(p[n:], c.buf[:l-n])
	}
	return n, nil
}

// This transparently resizes the buffer to hold at least minSize bytes.
// In general, we double the buffer size each time unless it's already bigger
// than 8K, in which case we only increase it by 25%.
// But if minSize is bigger than that number, we'll use minSize instead.
func (c *CircularBuffer) resize(minSize int) (int, error) {
	// first figure out how big it should be
	quarters := 8
	if len(c.buf) > 8192 {
		quarters = 5
	}
	newSize := len(c.buf) * quarters / 4
	if minSize > newSize {
		newSize = minSize
	}
	newbuf := make([]byte, newSize)
	_, err := c.peek(newbuf)
	if err != nil {
		// we didn't change anything
		return len(c.buf), err
	}
	// we now have a new, bigger buffer with all the contents in it
	c.buf = newbuf
	c.index = 0
	return len(c.buf), nil
}
