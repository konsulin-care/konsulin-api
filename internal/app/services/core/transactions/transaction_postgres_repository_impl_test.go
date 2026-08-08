package transactions

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"konsulin-service/internal/app/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ---------------------------------------------------------------------------
// Fake sql driver: returns a single Transaction row (or a query error).
// ---------------------------------------------------------------------------

type fakeDriver struct {
	fail bool
}

func (d *fakeDriver) Open(string) (driver.Conn, error) {
	return fakeConn{fail: d.fail}, nil
}

type fakeConn struct {
	fail bool
}

func (c fakeConn) Prepare(string) (driver.Stmt, error) { return fakeStmt{fail: c.fail}, nil }
func (fakeConn) Close() error                          { return nil }
func (fakeConn) Begin() (driver.Tx, error)             { return nil, errors.New("not supported") }

type fakeStmt struct {
	fail bool
}

func (fakeStmt) Close() error { return nil }
func (fakeStmt) NumInput() int { return -1 }
func (fakeStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, errors.New("not supported")
}

func (s fakeStmt) Query([]driver.Value) (driver.Rows, error) {
	return &fakeRows{fail: s.fail}, nil
}

type fakeRows struct {
	done bool
	fail bool
}

func (fakeRows) Columns() []string {
	return []string{
		"id", "patient_id", "practitioner_id", "payment_link", "status_payment",
		"amount", "currency", "created_at", "updated_at", "session_total",
		"length_minutes_per_session", "session_type", "notes", "refund_status",
		"refund_amount", "audit_log",
	}
}

func (fakeRows) Close() error { return nil }

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	if r.fail {
		return errors.New("query failed")
	}
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	values := []driver.Value{
		"txn-1", "pat-1", "prac-1", "https://pay.example/1", "completed",
		float64(250000), "IDR", ts, ts, int64(4), int64(60), "online",
		"note", "none", float64(0), []byte(`{"by":"sys"}`),
	}
	copy(dest, values)
	return nil
}

var (
	fakeDriversMu sync.Mutex
	fakeDrivers   = map[string]*fakeDriver{}
)

// newFakeDB returns a *sql.DB backed by a fake driver registered once under
// the given name. Set fail to make the query return an error.
func newFakeDB(t *testing.T, name string, fail bool) *sql.DB {
	t.Helper()
	fakeDriversMu.Lock()
	drv, ok := fakeDrivers[name]
	if !ok {
		drv = &fakeDriver{}
		sql.Register(name, drv)
		fakeDrivers[name] = drv
	}
	fakeDriversMu.Unlock()
	drv.fail = fail
	db, err := sql.Open(name, "")
	require.NoError(t, err)
	return db
}

// captureLogger returns a zap logger writing to an in-memory buffer.
func captureLogger() (*zap.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	encoder := zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zapcore.DebugLevel)
	return zap.New(core), &buf
}

func newTestRepo(t *testing.T, name string, fail bool) (*transactionPostgresRepository, *bytes.Buffer) {
	t.Helper()
	db := newFakeDB(t, name, fail)
	logger, buf := captureLogger()
	return &transactionPostgresRepository{DB: db, Log: logger}, buf
}

// ---------------------------------------------------------------------------
// CreateTransaction / UpdateTransaction behavior
// ---------------------------------------------------------------------------

func TestCreateTransactionSuccess(t *testing.T) {
	repo, buf := newTestRepo(t, "txn-ok", false)

	tx, err := repo.CreateTransaction(context.Background(), &models.Transaction{ID: "txn-1"})
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.Equal(t, "txn-1", tx.ID)
	require.Equal(t, "pat-1", tx.PatientID)
	require.Equal(t, "completed", string(tx.StatusPayment))
	require.Equal(t, 250000.0, tx.Amount)
	require.Equal(t, "IDR", tx.Currency)
	require.Equal(t, 4, tx.SessionTotal)

	logs := buf.String()
	require.Contains(t, logs, "transactionPostgresRepository.CreateTransaction called")
	require.Contains(t, logs, "transactionPostgresRepository.CreateTransaction succeeded")
}

func TestCreateTransactionDBError(t *testing.T) {
	repo, buf := newTestRepo(t, "txn-err", true)

	tx, err := repo.CreateTransaction(context.Background(), &models.Transaction{ID: "txn-1"})
	require.Nil(t, tx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "query failed")

	logs := buf.String()
	require.Contains(t, logs, "transactionPostgresRepository.CreateTransaction called")
	require.Contains(t, logs, "transactionPostgresRepository.CreateTransaction error executing insert")
}

func TestUpdateTransactionSuccess(t *testing.T) {
	repo, buf := newTestRepo(t, "txn-upd-ok", false)

	tx, err := repo.UpdateTransaction(context.Background(), &models.Transaction{ID: "txn-1", PatientID: "pat-1"})
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.Equal(t, "txn-1", tx.ID)
	require.Equal(t, "prac-1", tx.PractitionerID)

	logs := buf.String()
	require.Contains(t, logs, "transactionPostgresRepository.UpdateTransaction called")
	require.Contains(t, logs, "transactionPostgresRepository.UpdateTransaction succeeded")
}

func TestUpdateTransactionDBError(t *testing.T) {
	repo, buf := newTestRepo(t, "txn-upd-err", true)

	tx, err := repo.UpdateTransaction(context.Background(), &models.Transaction{ID: "txn-1"})
	require.Nil(t, tx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "query failed")

	logs := buf.String()
	require.Contains(t, logs, "transactionPostgresRepository.UpdateTransaction called")
	require.Contains(t, logs, "transactionPostgresRepository.UpdateTransaction error executing update")
}
