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
	ConnInfo string // host:port
	Db       int    // redis db identifier
	Conn     redis.Conn
}

func (tc *TracedConn) Close() error {
	return tc.Conn.Close()
}

func (tc *TracedConn) Err() error {
	return tc.Conn.Err()
}

func (tc *TracedConn) Do(cmdName string, args ...interface{}) (reply interface{}, err error) {
	if len(args) == 0 {
		return tc.Conn.Do(cmdName, args...)
	}

	var span ot.Span
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

	if _, ok := args[len(args)-1].(context.Context); !ok {
		span = ot.StartSpan("redis")
		return tc.Conn.Do(cmdName, args[:len(args)-1]...)
	}
	span, _ = ot.StartSpanFromContext(args[len(args)-1].(context.Context), "redis")
	return tc.Conn.Do(cmdName, args[:len(args)-1]...)
}

func (tc *TracedConn) Send(cmdName string, args ...interface{}) error {
	return tc.Conn.Send(cmdName, args...)
}

func (tc *TracedConn) Flush() error {
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
