package otredigo

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/garyburd/redigo/redis"
	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// TracedConn is a traced implementation of github.com/garyburd/redigo/redis.Conn
type TracedConn struct {
	ConnInfo         string // host:port
	Db               int    // redis db identifier
	Conn             redis.Conn
	span             ot.Span
	pipelineCommands []string
}

func (tc *TracedConn) Close() error {
	return tc.Conn.Close()
}

func (tc *TracedConn) Err() error {
	return tc.Conn.Err()
}

var redigoComponent = ot.Tag{string(ext.Component), "redigo"}

func (tc *TracedConn) Do(cmdName string, args ...interface{}) (reply interface{}, err error) {
	if len(args) == 0 {
		return tc.Conn.Do(cmdName, args...)
	}

	span, args := spanAndArgs(cmdName, args)
	defer func() {
		span.SetTag(string(ext.DBStatement), databaseStatement(cmdName, args[:len(args)-1]...))
		span.SetTag(string(ext.DBInstance), tc.ConnInfo)
		span.SetTag(string(ext.DBType), "redis")
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.SetTag("redis.error", err.Error())
		}
		span.Finish()
	}()
	return tc.Conn.Do(cmdName, args)
}

func spanAndArgs(cmdName string, args ...interface{}) (ot.Span, []interface{}) {
	var span ot.Span
	if _, ok := args[len(args)-1].(context.Context); !ok {
		span = ot.StartSpan("redis", redigoComponent)
		return span, args
	}
	span, _ = ot.StartSpanFromContext(args[len(args)-1].(context.Context), cmdName, redigoComponent)
	return span, args[:len(args)-1]
}

func (tc *TracedConn) Send(cmdName string, args ...interface{}) error {
	if tc.span == nil {
		tc.span, args = spanAndArgs(cmdName, args)
		return tc.Conn.Send(cmdName, args...)
	}
	return tc.Conn.Send(cmdName, args...)
}

func (tc *TracedConn) Flush() error {
	if tc.span != nil {
		tc.span.Finish()
	}
	return tc.Conn.Flush()
}

func (tc *TracedConn) Receive() (reply interface{}, err error) {
	return tc.Conn.Receive()
}

func ConnectTo(redisURL string) (c redis.Conn, err error) {
	URL, err := url.Parse(redisURL)
	if err != nil {
		return
	}

	dialOpts := make([]redis.DialOption, 0)

	if URL.User != nil {
		pw, _ := URL.User.Password()
		dialOpts = append(dialOpts, redis.DialPassword(pw))
	}
	var db int
	if len(URL.Path) > 1 {
		db, _ = strconv.Atoi(URL.Path[1:])
		dialOpts = append(dialOpts, redis.DialDatabase(db))
	}
	c, err = redis.Dial("tcp", URL.Host, dialOpts...)
	return &TracedConn{Conn: c, Db: db, ConnInfo: URL.Host}, err
}

func databaseStatement(cmd string, args ...interface{}) string {
	stmt := cmd
	for _, a := range args {
		stmt += fmt.Sprintf(" %v", a)
	}
	return stmt
}
