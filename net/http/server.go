package othttp

import (
	"fmt"
	"net/http"
	"strings"

	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/urfave/negroni"
)

func ServerMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// extract SpanContext from http headers carrier
		sc, err := ot.GlobalTracer().Extract(ot.HTTPHeaders, ot.HTTPHeadersCarrier(r.Header))
		var span ot.Span
		if err == nil {
			span = ot.GlobalTracer().StartSpan(getOperationName(r.Method, r.URL.String()), ot.ChildOf(sc))
		} else { // if we couldn't extract a SpanContext from the headers, proceed with serving the request using a new parent span
			span = ot.StartSpan(getOperationName(r.Method, r.URL.String()))
		}
		defer span.Finish()
		ctx := ot.ContextWithSpan(r.Context(), span)
		// Use negroni.Responsewriter, providing facilities for getting response information
		w = negroni.NewResponseWriter(w)
		h.ServeHTTP(w, r.WithContext(ctx))
		setHTTPTags(span, r, w)
	})
}

func setHTTPTags(span ot.Span, r *http.Request, w http.ResponseWriter) {
	span.SetTag(string(ext.SpanKind), string(ext.SpanKindRPCServerEnum))
	span.SetTag(string(ext.HTTPMethod), r.Method)
	span.SetTag(string(ext.HTTPUrl), r.URL.Path)
	span.SetTag(string(ext.PeerHostname), r.Host)
	rw := w.(negroni.ResponseWriter)
	span.SetTag(string(ext.HTTPStatusCode), rw.Status())
	span.SetTag("http.response_body.size", rw.Size())
}

func getOperationName(method, path string) string {
	spath := strings.Split(path, "/")
	if len(spath) <= 1 {
		path = "/"
	} else {
		path = "/" + spath[1]
	}
	return fmt.Sprintf("%s %s", method, path)
}
