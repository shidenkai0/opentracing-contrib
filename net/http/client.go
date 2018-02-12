package othttp

import (
	"net/http"

	"github.com/opentracing/opentracing-go/ext"

	ot "github.com/opentracing/opentracing-go"
)

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
	ext.SpanKindRPCClient.Set(span) // mark this span as a remote client call

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
	if resp.StatusCode/100 == 5 {
		span.SetTag(string(ext.Error), true)
	}
	return resp, err
}

// NewTracedHTTPClient returns a new traced http.Client
func NewTracedHTTPClient() *http.Client {
	return &http.Client{
		Transport: &TracedTransport{&http.Transport{}},
	}
}
