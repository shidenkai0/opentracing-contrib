package othttp

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/opentracing/opentracing-go/ext"

	ot "github.com/opentracing/opentracing-go"
)

// MaxContentLength is the maximum content length for which we'll read and capture
// the contents of the request body. Anything larger will still be traced but the
// body will not be captured as trace metadata.
const MaxContentLength = 1 << 16

// TracedTransport is a traced HTTP transport that captures http call spans
type TracedTransport struct {
	*http.Transport
}

// RoundTrip satisfies the RoundTripper interface, wraps the sub Transport and
// captures a span of the HTTP request.
func (t *TracedTransport) RoundTrip(r *http.Request) (resp *http.Response, err error) {
	span, ctx := ot.StartSpanFromContext(r.Context(), "http.request")
	r = r.WithContext(ctx)
	defer func() {
		if err != nil {
			span.SetTag("http.error", err.Error())
			span.SetTag(string(ext.Error), true)
		}
		span.Finish()
	}()

	span.SetTag(string(ext.HTTPMethod), r.Method)
	span.SetTag(string(ext.HTTPUrl), r.URL.String())

	contentLength, _ := strconv.Atoi(r.Header.Get("Content-Length"))
	if r.Body != nil && contentLength < MaxContentLength {
		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		span.SetTag(string(ext.DBStatement), string(buf))
		r.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
	}

	err = ot.GlobalTracer().Inject(span.Context(), ot.HTTPHeaders, ot.HTTPHeadersCarrier(r.Header))
	if err != nil {
		return nil, err
	}

	// execute standard roundtrip
	resp, err = t.Transport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	span.SetTag(string(ext.HTTPStatusCode), resp.StatusCode)
	return resp, err
}

// NewTracedHTTPClient returns a new traced http.Client
func NewTracedHTTPClient() *http.Client {
	return &http.Client{
		Transport: &TracedTransport{&http.Transport{}},
	}
}
