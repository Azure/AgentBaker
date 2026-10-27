package scenario

import (
	"bytes"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bastionWriteTestConn struct {
	net.Conn
	write func([]byte) (int, error)
}

func (c bastionWriteTestConn) Write(p []byte) (int, error) {
	return c.write(p)
}

func TestBastionWriteMessageSize(t *testing.T) {
	for _, size := range []int{0, 1, 4096, 4097, 9000} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			payload := bytes.Repeat([]byte("x"), size)
			var messages [][]byte
			tunnel := &tunnelSession{Conn: bastionWriteTestConn{
				write: func(p []byte) (int, error) {
					messages = append(messages, bytes.Clone(p))
					return len(p), nil
				},
			}}
			n, err := tunnel.Write(payload)
			require.NoError(t, err)
			assert.Equal(t, len(payload), n)
			assert.Equal(t, (size+4095)/4096, len(messages))
			assert.Equal(t, string(payload), string(bytes.Join(messages, nil)))
			for _, message := range messages {
				assert.LessOrEqual(t, len(message), 4096)
			}
		})
	}
}

func TestBastionWriteFailure(t *testing.T) {
	for _, test := range []struct {
		name    string
		n       int
		err     error
		wantErr error
	}{
		{name: "write error", err: io.ErrClosedPipe, wantErr: io.ErrClosedPipe},
		{name: "partial write error", n: 3, err: io.ErrClosedPipe, wantErr: io.ErrClosedPipe},
		{name: "short write", n: 3, wantErr: io.ErrShortWrite},
		{name: "zero progress", wantErr: io.ErrShortWrite},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			tunnel := &tunnelSession{Conn: bastionWriteTestConn{
				write: func(p []byte) (int, error) {
					calls++
					if calls == 2 {
						return test.n, test.err
					}
					return len(p), nil
				},
			}}
			n, err := tunnel.Write(make([]byte, 9000))
			assert.ErrorIs(t, err, test.wantErr)
			assert.Equal(t, 4096+test.n, n)
			assert.Equal(t, 2, calls)
		})
	}
}

func TestBastionWriteDoesNotInterleaveMessages(t *testing.T) {
	var messages [][]byte
	var mu sync.Mutex
	firstWrite, secondWrite, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)
	blockFirst := sync.OnceFunc(func() {
		close(firstWrite)
		<-release
	})
	signalSecond := sync.OnceFunc(func() { close(secondWrite) })
	tunnel := &tunnelSession{Conn: bastionWriteTestConn{
		write: func(p []byte) (int, error) {
			mu.Lock()
			messages = append(messages, bytes.Clone(p))
			mu.Unlock()
			if p[0] == 'a' {
				blockFirst()
			} else {
				signalSecond()
			}
			return len(p), nil
		},
	}}
	first := bytes.Repeat([]byte("a"), 9000)
	second := bytes.Repeat([]byte("b"), 9000)
	var group sync.WaitGroup
	for _, payload := range [][]byte{first, second} {
		group.Go(func() {
			n, err := tunnel.Write(payload)
			assert.NoError(t, err)
			assert.Equal(t, len(payload), n)
		})
		<-firstWrite
	}
	select {
	case <-secondWrite:
		t.Error("second write started before the first write finished")
	case <-time.After(100 * time.Millisecond):
	}
	unblock()
	group.Wait()
	assert.Equal(t, 6, len(messages))
	assert.True(t, bytes.Equal(append(first, second...), bytes.Join(messages, nil)))
}
