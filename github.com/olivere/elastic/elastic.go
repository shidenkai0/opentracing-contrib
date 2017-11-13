package elastictrace

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strconv"

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
func (t *TracedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	span, ctx := ot.StartSpanFromContext(r.Context(), "elastic.query")
	r = r.WithContext(ctx)
	defer span.Finish()
	span.SetTag("elasticsearch.method", r.Method)
	span.SetTag("elasticsearch.url", r.URL.Path)
	span.SetTag("elasticsearch.params", r.URL.Query().Encode())

	contentLength, _ := strconv.Atoi(r.Header.Get("Content-Length"))
	if r.Body != nil && contentLength < MaxContentLength {
		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		span.SetTag("elasticsearch.body", buf)
		r.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
	}

	// execute standard roundtrip
	return t.Transport.RoundTrip(r)
}

// NewTracedHTTPClient returns a new http.Client with a custom transport dedicated to use with github.com/olivere/elastic
func NewTracedHTTPClient(service string) *http.Client {
	return &http.Client{
		Transport: &TracedTransport{&http.Transport{}},
	}
}
