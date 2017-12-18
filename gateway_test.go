package rap

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const srvAddr string = "127.0.0.1:10111"

type gwTester struct {
	isServed   bool
	haveServed chan struct{}
}

func newGWTester() *gwTester {
	return &gwTester{
		haveServed: make(chan struct{}),
	}
}

func (gt *gwTester) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
	if !gt.isServed && gt.haveServed != nil {
		gt.isServed = true
		close(gt.haveServed)
	}
}

func Test_Gateway_ListenAndServe(t *testing.T) {
	gt := newGWTester()
	srv := &Server{
		Addr:    srvAddr,
		Handler: gt,
	}
	go srv.ListenAndServe()
	gw := NewGateway(srvAddr)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	gw.ServeHTTP(rr, r)
	<-gt.haveServed
	srv.listener.Close()
}

func Test_Gateway_simple(t *testing.T) {
	gt := newGWTester()
	srv := &Server{
		Handler: gt,
	}
	ln, err := srv.Listen(srvAddr)
	assert.NoError(t, err)
	srv.listener = ln
	go srv.ListenAndServe()
	defer ln.Close()
	gw := NewGateway(srvAddr)
	assert.NotNil(t, gw)

	// send simple request
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	gw.ServeHTTP(rr, r)
	assert.Equal(t, http.StatusOK, rr.Code)

	// send request with body
	rr = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/", bytes.NewBuffer([]byte{0x20, 0x20}))
	gw.ServeHTTP(rr, r)
	assert.Equal(t, http.StatusOK, rr.Code)

	// send request with large body
	rr = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/", bytes.NewBuffer(make([]byte, 0x10000)))
	gw.ServeHTTP(rr, r)
	assert.Equal(t, http.StatusOK, rr.Code)

	// send request for websocket upgrade
	// fails since http hijacker not supported by httptest.ResponseRecorder
	rr = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Add("Upgrade", "websocket")
	r.Header.Add("Connection", "upgrade")
	gw.ServeHTTP(rr, r)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func Test_Gateway_no_answer(t *testing.T) {
	gw := NewGateway(noSrvAddr)
	gw.Client.DialTimeout = time.Millisecond * 10
	// send simple request
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	gw.ServeHTTP(rr, r)
	assert.Equal(t, http.StatusGatewayTimeout, rr.Code)
}
