package othttp

import (
	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

var httpComponent = ot.Tag{string(ext.Component), "http"}
