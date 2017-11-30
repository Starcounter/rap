package rap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_FramePool_FrameDataAlloc(t *testing.T) {
	fd1 := FrameDataAlloc()
	FrameDataFree(fd1)
	fd2 := FrameDataAlloc()
	FrameDataFree(fd2)
}

func Test_FramePool_FrameDataAllocID(t *testing.T) {
	fd1 := FrameDataAllocID(0x123)
	assert.Equal(t, ExchangeID(0x123), fd1.Header().ExchangeID())
	fd2 := FrameDataAllocID(0x012)
	assert.Equal(t, ExchangeID(0x12), fd2.Header().ExchangeID())
	FrameDataFree(fd1)
	FrameDataFree(fd2)
}
