package graph

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"
	"sync"
	"testing"

	"github.com/lib/pq"
)

// The cost these tests measure is "how many statements did the store send", and nothing in
// the store reports that. Wrapping the driver is the only vantage point that sees every
// path — QueryRowContext, QueryContext, ExecContext, inside a transaction or not — without
// asking the store to instrument itself for a test.
//
// One counter serves the whole package: registered drivers are global, and these tests are
// sequential, so there is nothing to key a per-env counter on that is worth the machinery.
const countingDriverName = "postgres-counting"

var (
	registerCountingDriver sync.Once
	packageSQLCounter      sqlCounter
)

// sqlCounter records the SQL text of every statement sent while recording is on.
type sqlCounter struct {
	mu        sync.Mutex
	recording bool
	stmts     []string
}

func (c *sqlCounter) record(query string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.recording {
		c.stmts = append(c.stmts, query)
	}
}

// measure runs fn with recording on and returns every statement it caused. The websocket
// tests below call it around a push, so it has to survive statements arriving from another
// goroutine — hence the lock rather than a plain slice append.
func (c *sqlCounter) measure(fn func()) statements {
	c.mu.Lock()
	c.recording, c.stmts = true, nil
	c.mu.Unlock()

	fn()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.recording = false
	out := make(statements, len(c.stmts))
	copy(out, c.stmts)
	c.stmts = nil
	return out
}

// statements is one measurement: the SQL the store sent, in order.
type statements []string

// touching counts the statements that read the named relation. Totals move whenever
// anything unrelated changes; a count of the queries that actually hit game_modes or
// game_sessions is the part a regression would show up in.
func (s statements) touching(relation string) int {
	n := 0
	for _, stmt := range s {
		if strings.Contains(stmt, relation) {
			n++
		}
	}
	return n
}

func (s statements) String() string {
	return strings.Join(s, "\n---\n")
}

type countingDriver struct{ inner driver.Driver }

func (d countingDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return countingConn{inner: conn}, nil
}

// countingConn forwards to pq and counts what goes past. Query/ExecContext are the paths
// database/sql prefers, but it falls back to preparing when a driver answers ErrSkip, so
// the prepared statement is wrapped too — and counted only where the direct call was not,
// which is what keeps a fallback from counting twice.
type countingConn struct{ inner driver.Conn }

func (c countingConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.inner.Prepare(query)
	if err != nil {
		return nil, err
	}
	return countingStmt{inner: stmt, query: query}, nil
}

func (c countingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	preparer, ok := c.inner.(driver.ConnPrepareContext)
	if !ok {
		return c.Prepare(query)
	}
	stmt, err := preparer.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return countingStmt{inner: stmt, query: query}, nil
}

func (c countingConn) Close() error              { return c.inner.Close() }
func (c countingConn) Begin() (driver.Tx, error) { return c.inner.Begin() }

func (c countingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if beginner, ok := c.inner.(driver.ConnBeginTx); ok {
		return beginner.BeginTx(ctx, opts)
	}
	return c.inner.Begin()
}

func (c countingConn) Ping(ctx context.Context) error {
	if pinger, ok := c.inner.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c countingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.inner.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	rows, err := queryer.QueryContext(ctx, query, args)
	if err == driver.ErrSkip {
		return nil, err
	}
	packageSQLCounter.record(query)
	return rows, err
}

func (c countingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.inner.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	res, err := execer.ExecContext(ctx, query, args)
	if err == driver.ErrSkip {
		return nil, err
	}
	packageSQLCounter.record(query)
	return res, err
}

type countingStmt struct {
	inner driver.Stmt
	query string
}

func (s countingStmt) Close() error  { return s.inner.Close() }
func (s countingStmt) NumInput() int { return s.inner.NumInput() }

func (s countingStmt) Exec(args []driver.Value) (driver.Result, error) {
	packageSQLCounter.record(s.query)
	return s.inner.Exec(args)
}

func (s countingStmt) Query(args []driver.Value) (driver.Rows, error) {
	packageSQLCounter.record(s.query)
	return s.inner.Query(args)
}

// newCountingIntegrationEnv is newQueueIntegrationEnv over the counting driver.
func newCountingIntegrationEnv(t *testing.T) (*queueIntegrationEnv, *sqlCounter) {
	t.Helper()
	registerCountingDriver.Do(func() {
		sql.Register(countingDriverName, countingDriver{inner: &pq.Driver{}})
	})
	return newQueueIntegrationEnvWithDriver(t, countingDriverName), &packageSQLCounter
}
