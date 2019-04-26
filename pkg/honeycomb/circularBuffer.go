package honeycomb

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
)

// CircularBuffer is an io.ReadWriter that implements a...you guessed it, circular
// buffer of bytes. You set the buffer size, and you can write to it until it's full,
// and then you can read from it until it's empty again. It's safe to use in concurrent
// operations.
//
// Its purpose is to provide a safe way to pass an io.Writer to a separate process for its
// stdout or stderr, and to have a local goroutine reading and parsing the stream of data in
// real time.
//
// The C member is a chan struct{} that receives a new value for every operation that changes
// the buffer's length to a nonzero value (every successful nonempty Write and any partial Consume).
//
// The Peek operation is a read without advancing the read pointer. This can be
// useful in parsing situations.
//
// Calling Close() prevents further writes, and after the entire input has been consumed, returns EOF
// for further read operations.
//
// Once created, the size of a CircularBuffer can never be changed. The size of the buffer should be
// larger than the largest single write you expect in one call to Write(), and twice that is
// a good idea.
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

// ErrNoRoom is the error returned when a Write doesn't have enough space to write the entire input.
var ErrNoRoom = errors.New("insufficient capacity")

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
// buffer, no bytes are written, the buffer is not modified at all and the call returns ErrNoRoom.
func (c *CircularBuffer) Write(p []byte) (int, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.closed {
		return 0, io.EOF
	}
	if len(p) > len(c.buf)-c.len {
		return 0, ErrNoRoom
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
// Peek() followed by Consume().
func (c *CircularBuffer) Read(p []byte) (int, error) {
	n, err := c.Peek(p)
	if err != nil {
		return n, err
	}
	return c.Consume(n), nil
}

// Peek retrieves the leading bytes from the buffer but does not move the index
// pointer. It returns as many bytes as the maximum of the length of p or the current
// length of the buffer.
func (c *CircularBuffer) Peek(p []byte) (int, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
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

// Consume advances the index pointer by n bytes, or by the current length of the input,
// whichever is shorter. It returns the number of bytes actually advanced.
func (c *CircularBuffer) Consume(n int) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if n > c.len {
		n = c.len
	}
	c.index = (c.index + n) % len(c.buf)
	c.addLen(-n)
	return n
}

// Capacity returns the capacity of the buffer; this is the value set
// at initialization.
func (c *CircularBuffer) Capacity() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return len(c.buf)
}

// Listen returns a channel that will receive an event whenever the c.Len() changes
// and the result is nonzero. In other words, listening to this channel will tell
// you whenever there is something in the buffer to read. However, redundant notifications
// will be dropped on the floor. So if you call write 3 times but don't read from the
// channel until later, you'll only see a single event.
// Calling Close() also closes this channel.
func (c *CircularBuffer) Listen() chan struct{} {
	return c.C
}

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
