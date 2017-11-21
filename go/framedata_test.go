package rap

import (
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFrameData(t *testing.T) {
	fd := NewFrameData()
	assert.NotNil(t, fd)
	assert.Equal(t, FrameHeaderSize, fd.Buffered())
	assert.Equal(t, FrameMaxSize-FrameHeaderSize, fd.Available())
}

func TestNewFrameDataExchangeIDRange(t *testing.T) {
	fd := NewFrameData()
	assert.Equal(t, ExchangeID(0), fd.Header().ExchangeID())
	fd.WriteHeader(ExchangeID(1))
	assert.Equal(t, ExchangeID(1), fd.Header().ExchangeID())
	fd.WriteHeader(MaxExchangeID)
	assert.Equal(t, MaxExchangeID, fd.Header().ExchangeID())
	assert.Panics(t, func() { fd.WriteHeader(ExchangeID(0xFFFF)) })
}

func TestFrameDataString(t *testing.T) {
	fd := NewFrameData()
	fd.WriteString("Hello world")
	assert.Equal(t, "[FrameData [FrameHeader [ExchangeID 0000] ... 0 (16)] 0b48656c6c6f20776f726c64]", fd.String())
	fd.WriteString("the data is greater than 32 length")
	assert.Equal(t, "[FrameData [FrameHeader [ExchangeID 0000] ... 0 (51)] 0b48656c6c6f20776f726c6422746865206461746120697320677265...]", fd.String())
}

type shortWriter struct {
	w io.Writer
	n int64
}

func (t *shortWriter) Write(p []byte) (n int, err error) {
	if t.n <= 0 {
		return 0, io.ErrShortWrite
	}
	// real write
	n = len(p)
	if int64(n) > t.n {
		n = int(t.n)
	}
	n, err = t.w.Write(p[0:n])
	t.n -= int64(n)
	return
}

func TestFrameDataPayloadAndWriteTo(t *testing.T) {
	fd := NewFrameData()
	assert.NotNil(t, fd.Payload())
	assert.Equal(t, 0, len(fd.Payload()))
	ba := make([]byte, 0, FrameMaxPayloadSize)
	assert.Panics(t, func() { FrameData(ba).WriteTo(ioutil.Discard) })
	for i := 0; i < FrameMaxPayloadSize; i++ {
		b := byte(i % 0xff)
		fd.WriteByte(b)
		ba = append(ba, b)
	}
	assert.Equal(t, ba, fd.Payload())
	fd.Header().SetBody()

	_, err := fd.WriteTo(&shortWriter{ioutil.Discard, 1})
	assert.Equal(t, io.ErrShortWrite, err)

	_, err = fd.WriteTo(ioutil.Discard)
	assert.NoError(t, err)
	fd.WriteByte(0x00)

	_, err = fd.WriteTo(ioutil.Discard)
	assert.Equal(t, ErrFrameTooBig, err)
}

func TestFrameDataUint64(t *testing.T) {
	for i := uint(0); i < 64; i++ {
		n := (uint64(1) << i) - 1
		for j := uint64(0); j < 3; j++ {
			fd := NewFrameData()
			fd.WriteUint64(n + j)
			fr := NewFrameReader(fd)
			assert.Equal(t, n+j, fr.ReadUint64())
		}
	}
}

func TestFrameDataInt64(t *testing.T) {
	for i := uint(0); i < 64; i++ {
		for j := int64(0); j < 3; j++ {
			for k := int64(-1); k < 2; k += 2 {
				n := (((int64(1) << i) - 1) + j) * k
				fd := NewFrameData()
				fd.WriteInt64(n)
				fr := NewFrameReader(fd)
				assert.Equal(t, n, fr.ReadInt64())
			}
		}
	}
}
