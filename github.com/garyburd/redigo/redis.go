package otredigo

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/garyburd/redigo/redis"
	ot "github.com/opentracing/opentracing-go"
)

// TracedConn is a traced implementation of github.com/garyburd/redigo/redis.Conn
type TracedConn struct {
	Conn redis.Conn
}

func (tc *TracedConn) Close() error {
	return tc.Conn.Close()
}

func (tc *TracedConn) Err() error {
	return tc.Conn.Err()
}

func (tc *TracedConn) Do(cmdName string, args ...interface{}) (reply interface{}, err error) {
	if len(args) == 0 {
		return nil, errors.New("otredigo: called redis.Conn.Do with no arguments")
	}
	if _, ok := args[0].(context.Context); !ok {
		span := ot.StartSpan(fmt.Sprintf("redis.%s", cmdName))
		defer span.Finish()
		span.SetTag(fmt.Sprintf("redis.%s.Args", cmdName), args)
		return tc.Conn.Do(cmdName, args...)
	}
	span, _ := ot.StartSpanFromContext(args[0].(context.Context), fmt.Sprintf("redis.%s", cmdName))
	defer span.Finish()
	span.SetTag(fmt.Sprintf("redis.%s.Args", cmdName), args)

	return tc.Conn.Do(cmdName, args...)
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

	if len(URL.Path) > 1 {
		db, _ := strconv.Atoi(URL.Path[1:])
		dialOpts = append(dialOpts, redis.DialDatabase(db))
	}
	return redis.Dial("tcp", URL.Host, dialOpts...)
}
