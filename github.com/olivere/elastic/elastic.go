package otelastic

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

// TracedTransport is a traced HTTP transport that captures Elasticsearch spans.
type TracedTransport struct {
	*http.Transport
}

// RoundTrip satisfies the RoundTripper interface, wraps the sub Transport and
// captures a span of the Elasticsearch request.
func (t *TracedTransport) RoundTrip(r *http.Request) (resp *http.Response, err error) {
	span, ctx := ot.StartSpanFromContext(r.Context(), "elastic.query")
	r = r.WithContext(ctx)
	defer func() {
		if err != nil {
			span.SetTag("elastic.error", err.Error())
			span.SetTag(string(ext.Error), true)
		}
		span.Finish()
	}()

	span.SetTag(string(ext.DBType), "elasticsearch")
	span.SetTag(string(ext.DBInstance), r.URL.Host)
	span.SetTag("elastic.method", r.Method)
	span.SetTag("elastic.url", r.URL.Path)
	span.SetTag("elastic.params", r.URL.Query().Encode())

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
	span.SetTag("elastic.status_code", resp.StatusCode)
	return resp, err
}

// NewTracedHTTPClient returns a new http.Client with a custom transport dedicated to use with github.com/olivere/elastic
func NewTracedHTTPClient() *http.Client {
	return &http.Client{
		Transport: &TracedTransport{&http.Transport{}},
	}
}
