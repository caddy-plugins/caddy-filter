package filter

import (
	"io"
	"net/http"
	"strconv"

	"github.com/admpub/caddy/caddyhttp/fastcgi"
	"github.com/admpub/caddy/caddyhttp/httpserver"
)

const defaultMaxBufferSize = 10 * 1024 * 1024

type filterHandler struct {
	next              httpserver.Handler
	rules             []*rule
	maximumBufferSize int
}

func (instance filterHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) (int, error) {
	// Do not intercept if this is a websocket upgrade request.
	if request.Method == "GET" && request.Header.Get("Upgrade") == "websocket" {
		return instance.next.ServeHTTP(writer, request)
	}
	return RewriteResponse(writer, request, func(wrapper http.ResponseWriter) bool {
		header := wrapper.Header()
		for _, rule := range instance.rules {
			if rule.matches(request, &header) {
				return true
			}
		}
		return false
	}, func(wrapper http.ResponseWriter) (bool, []byte) {
		var body []byte
		var bodyRetrieved bool
		header := wrapper.Header()
		wrapperd := wrapper.(*responseWriterWrapper)
		for _, rule := range instance.rules {
			if rule.matches(request, &header) {
				if !bodyRetrieved {
					body = wrapperd.RecordedAndDecodeIfRequired()
					bodyRetrieved = true
				}
				body = rule.execute(request, &header, body)
			}
		}
		return bodyRetrieved, body
	}, instance.maximumBufferSize, instance.next)
}

func RewriteResponse(writer http.ResponseWriter, request *http.Request,
	beforeFirstWrite func(http.ResponseWriter) bool,
	bodyIsRetrieved func(http.ResponseWriter) (bool, []byte),
	maximumBufferSize int, next httpserver.Handler) (int, error) {

	wrapper := newResponseWriterWrapperFor(writer, beforeFirstWrite)
	wrapper.maximumBufferSize = maximumBufferSize
	result, err := next.ServeHTTP(wrapper, request)
	if wrapper.skipped {
		return result, err
	}
	var logError error
	if err != nil {
		var ok bool
		// This handles https://github.com/echocat/caddy-filter/issues/4
		// If the fastcgi module is used and the FastCGI server produces log output
		// this is send (by the FastCGI module) as an error. We have to check this and
		// handle this case of error in a special way.
		if logError, ok = err.(fastcgi.LogError); !ok {
			return result, err
		}
	}
	if !wrapper.isInterceptingRequired() || !wrapper.isBodyAllowed() {
		wrapper.writeHeadersToDelegate(result)
		return result, logError
	}
	if !wrapper.isBodyAllowed() {
		return result, logError
	}
	bodyRetrieved, body := bodyIsRetrieved(wrapper)
	var n int
	if bodyRetrieved {
		oldContentLength := wrapper.Header().Get("Content-Length")
		if len(oldContentLength) > 0 {
			newContentLength := strconv.Itoa(len(body))
			wrapper.Header().Set("Content-Length", newContentLength)
		}
		n, err = wrapper.writeToDelegateAndEncodeIfRequired(body, result)
	} else {
		n, err = wrapper.writeRecordedToDelegate(result)
	}
	if err != nil {
		return result, err
	}
	if n < len(body) {
		return result, io.ErrShortWrite
	}
	return result, logError
}
