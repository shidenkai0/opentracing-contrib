package othttp

import (
	"fmt"
	"net/http"
	"strings"

	ot "github.com/opentracing/opentracing-go"
	"github.com/urfave/negroni"
)

func ServerMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// extract SpanContext from http headers carrier
		sc, err := ot.GlobalTracer().Extract(ot.HTTPHeaders, ot.HTTPHeadersCarrier(r.Header))
		// if we couldn't extract a SpanContext from the headers, proceed with serving the request using a new parent span
		if err != nil {
			span := ot.GlobalTracer().StartSpan(getOperationName(r.Method, r.URL.String()))
			defer span.Finish()
			ctx := ot.ContextWithSpan(r.Context(), span)
			h.ServeHTTP(w, r.WithContext(ctx))
			setHTTPTags(span, r, w)
			return
		}
		span := ot.GlobalTracer().StartSpan(getOperationName(r.Method, r.URL.String()), ot.ChildOf(sc))
		defer span.Finish()
		ctx := ot.ContextWithSpan(r.Context(), span)
		// Use negroni.Responsewriter, providing facilities for getting response information
		w = negroni.NewResponseWriter(w)
		h.ServeHTTP(w, r.WithContext(ctx))
		setHTTPTags(span, r, w)
	})
}

func setHTTPTags(span ot.Span, r *http.Request, w http.ResponseWriter) {
	span.SetTag("http.method", r.Method)
	span.SetTag("http.url", r.URL.String())
	rw := w.(negroni.ResponseWriter)
	span.SetTag("http.status_code", rw.Status())
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
