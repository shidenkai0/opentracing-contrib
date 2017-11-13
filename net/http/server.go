package othttp

import (
	"net/http"

	ot "github.com/opentracing/opentracing-go"
)

func ServerMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// extract SpanContext from http headers carrier
		sc, err := ot.GlobalTracer().Extract(ot.HTTPHeaders, ot.HTTPHeadersCarrier(r.Header))
		// if we couldn't extract a SpanContext from the headers, proceed with serving the request using a new parent span
		if err != nil {
			span := ot.GlobalTracer().StartSpan("http.request")
			defer span.Finish()
			ctx := ot.ContextWithSpan(r.Context(), span)
			h.ServeHTTP(w, r.WithContext(ctx))
			setHTTPTags(span, r)
			return
		}
		span := ot.GlobalTracer().StartSpan("http.request", ot.ChildOf(sc))
		defer span.Finish()
		ctx := ot.ContextWithSpan(r.Context(), span)
		h.ServeHTTP(w, r.WithContext(ctx))
		setHTTPTags(span, r)
	})
}

func setHTTPTags(span ot.Span, r *http.Request) {
	span.SetTag("http.method", r.Method)
	span.SetTag("http.url", r.URL.String())
	span.SetTag("http.status_code", r.Response.StatusCode)
}
