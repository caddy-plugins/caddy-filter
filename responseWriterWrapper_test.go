package filter

import (
	"bytes"
	"net/http"
	"reflect"

	. "gopkg.in/check.v1"
)

type responseWriterWrapperTest struct{}

func init() {
	Suite(&responseWriterWrapperTest{})
}

func (s *responseWriterWrapperTest) Test_newResponseWriterWrapperFor(c *C) {
	original := newMockResponseWriter()
	beforeFirstWrite := func(http.ResponseWriter) bool {
		return false
	}
	wrapper := newResponseWriterWrapperFor(original, beforeFirstWrite)
	c.Assert(wrapper.delegate, DeepEquals, original)
	c.Assert(wrapper.buffer, IsNil)
	c.Assert(reflect.ValueOf(wrapper.beforeFirstWrite), Equals, reflect.ValueOf(beforeFirstWrite))
	c.Assert(wrapper.bodyAllowed, Equals, true)
	c.Assert(wrapper.firstContentWritten, Equals, false)
}

func (s *responseWriterWrapperTest) Test_Header(c *C) {
	original := newMockResponseWriter()
	original.header.Add("a", "1")
	original.header.Add("b", "2")
	wrapper := newResponseWriterWrapperFor(original, nil)

	c.Assert(wrapper.Header().Get("a"), Equals, "1")
	c.Assert(wrapper.Header().Get("b"), Equals, "2")
	c.Assert(wrapper.Header().Get("c"), Equals, "")

	wrapper.Header().Del("a")
	wrapper.Header().Add("c", "3")
	c.Assert(wrapper.Header().Get("a"), Equals, "")
	c.Assert(wrapper.Header().Get("c"), Equals, "3")
}

func (s *responseWriterWrapperTest) Test_WriteHeader(c *C) {
	original := newMockResponseWriter()
	wrapper := newResponseWriterWrapperFor(original, nil)

	c.Assert(original.status, Equals, 0)

	wrapper.WriteHeader(200)
	c.Assert(wrapper.statusSetAtDelegate, Equals, 200)
	c.Assert(original.status, Equals, 0)
	c.Assert(wrapper.bodyAllowed, Equals, true)

	wrapper.WriteHeader(204)
	c.Assert(wrapper.statusSetAtDelegate, Equals, 204)
	c.Assert(original.status, Equals, 0)
	c.Assert(wrapper.bodyAllowed, Equals, false)
}

func (s *responseWriterWrapperTest) Test_bodyAllowedForStatus(c *C) {
	c.Assert(bodyAllowedForStatus(200), Equals, true)
	c.Assert(bodyAllowedForStatus(208), Equals, true)
	c.Assert(bodyAllowedForStatus(404), Equals, true)
	c.Assert(bodyAllowedForStatus(500), Equals, true)
	c.Assert(bodyAllowedForStatus(503), Equals, true)
	for i := 100; i < 200; i++ {
		c.Assert(bodyAllowedForStatus(i), Equals, false)
	}
	c.Assert(bodyAllowedForStatus(204), Equals, false)
	c.Assert(bodyAllowedForStatus(304), Equals, false)
}

func (s *responseWriterWrapperTest) Test_WriteWithoutRecording(c *C) {
	beforeFirstWriteCalled := false
	original := newMockResponseWriter()
	beforeFirstWrite := func(http.ResponseWriter) bool {
		beforeFirstWriteCalled = true
		return false
	}
	wrapper := newResponseWriterWrapperFor(original, beforeFirstWrite)
	len, err := wrapper.Write([]byte(""))
	c.Assert(len, Equals, 0)
	c.Assert(err, IsNil)
	c.Assert(wrapper.firstContentWritten, Equals, false)
	c.Assert(beforeFirstWriteCalled, Equals, false)

	len, err = wrapper.Write([]byte("foo"))
	c.Assert(len, Equals, 3)
	c.Assert(err, IsNil)
	c.Assert(wrapper.firstContentWritten, Equals, true)
	c.Assert(beforeFirstWriteCalled, Equals, true)

	len, err = wrapper.Write([]byte("bar"))
	c.Assert(len, Equals, 3)
	c.Assert(err, IsNil)

	c.Assert(original.buffer.Bytes(), DeepEquals, []byte("foobar"))
	c.Assert(wrapper.buffer, IsNil)
	c.Assert(wrapper.firstContentWritten, Equals, true)
	c.Assert(wrapper.isBodyAllowed(), Equals, true)
	c.Assert(wrapper.recorded(), DeepEquals, []byte{})
	c.Assert(wrapper.wasSomethingRecorded(), Equals, false)
}

func (s *responseWriterWrapperTest) Test_WriteWithRecording(c *C) {
	beforeFirstWriteCalled := false
	original := newMockResponseWriter()
	beforeFirstWrite := func(http.ResponseWriter) bool {
		beforeFirstWriteCalled = true
		return true
	}
	wrapper := newResponseWriterWrapperFor(original, beforeFirstWrite)
	len, err := wrapper.Write([]byte(""))
	c.Assert(len, Equals, 0)
	c.Assert(err, IsNil)
	c.Assert(wrapper.firstContentWritten, Equals, false)
	c.Assert(beforeFirstWriteCalled, Equals, false)

	len, err = wrapper.Write([]byte("foo"))
	c.Assert(len, Equals, 3)
	c.Assert(err, IsNil)
	c.Assert(wrapper.firstContentWritten, Equals, true)
	c.Assert(beforeFirstWriteCalled, Equals, true)

	len, err = wrapper.Write([]byte("bar"))
	c.Assert(len, Equals, 3)
	c.Assert(err, IsNil)

	c.Assert(original.buffer.Bytes(), DeepEquals, []byte(nil))
	c.Assert(wrapper.buffer.Bytes(), DeepEquals, []byte("foobar"))
	c.Assert(wrapper.firstContentWritten, Equals, true)
	c.Assert(wrapper.isBodyAllowed(), Equals, true)
	c.Assert(wrapper.recorded(), DeepEquals, []byte("foobar"))
	c.Assert(wrapper.wasSomethingRecorded(), Equals, true)
}

func (s *responseWriterWrapperTest) Test_WriteWithBufferOverflow(c *C) {
	original := newMockResponseWriter()
	beforeFirstWrite := func(http.ResponseWriter) bool {
		return true
	}
	wrapper := newResponseWriterWrapperFor(original, beforeFirstWrite)
	wrapper.maximumBufferSize = 5
	wrapper.Write([]byte("foo"))
	c.Assert(wrapper.wasSomethingRecorded(), Equals, true)
	c.Assert(wrapper.recorded(), DeepEquals, []byte("foo"))
	c.Assert(original.buffer.Bytes(), DeepEquals, []byte(nil))

	wrapper.Write([]byte("bar"))
	c.Assert(wrapper.wasSomethingRecorded(), Equals, false)
	c.Assert(wrapper.recorded(), DeepEquals, []byte{})
	c.Assert(original.buffer.Bytes(), DeepEquals, []byte("foobar"))
}

///////////////////////////////////////////////////////////////////////////////////////////
// MOCKS
///////////////////////////////////////////////////////////////////////////////////////////

func newMockResponseWriter() *mockResponseWriter {
	result := new(mockResponseWriter)
	result.header = http.Header{}
	result.buffer = new(bytes.Buffer)
	return result
}

type mockResponseWriter struct {
	header http.Header
	status int
	buffer *bytes.Buffer
	error  error
}

func (instance *mockResponseWriter) Header() http.Header {
	return instance.header
}

func (instance *mockResponseWriter) WriteHeader(status int) {
	instance.status = status
}

func (instance *mockResponseWriter) Write(content []byte) (int, error) {
	if instance.error != nil {
		return 0, instance.error
	}
	return instance.buffer.Write(content)
}
