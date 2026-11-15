package whatsmeow_service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.mau.fi/whatsmeow/store/sqlstore"
	_ "modernc.org/sqlite"

	"github.com/evolution-foundation/evolution-go/pkg/config"
)

// TestAuthContainerGetIsSingleAndConcurrent garante que o holder cria o
// container uma única vez, mesmo com 50 goroutines concorrentes: se `get`
// voltasse a chamar o callback a cada chamada (o comportamento antigo, um pool
// por StartClient), o contador seria > 1 e os ponteiros divergiriam.
func TestAuthContainerGetIsSingleAndConcurrent(t *testing.T) {
	holder := newAuthContainer()

	var calls int32
	created := &sqlstore.Container{}
	open := func() (*sqlstore.Container, error) {
		atomic.AddInt32(&calls, 1)
		// Segura a primeira execução para que as demais goroutines cheguem
		// enquanto o sync.Once ainda não terminou.
		time.Sleep(20 * time.Millisecond)
		return created, nil
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	results := make([]*sqlstore.Container, goroutines)
	errs := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = holder.get(open)
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("open callback ran %d times, want exactly 1", got)
	}
	for i := range results {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: unexpected error: %v", i, errs[i])
		}
		if results[i] != created {
			t.Fatalf("goroutine %d: got container %p, want %p (same pointer for all callers)", i, results[i], created)
		}
		if results[i] != holder.current() {
			t.Fatalf("current() = %p, want %p", holder.current(), created)
		}
	}
}

// TestAuthContainerGetMemoizesError garante que a falha de criação não é
// retentada por chamada: todas recebem o mesmo erro e o callback roda 1 vez.
func TestAuthContainerGetMemoizesError(t *testing.T) {
	holder := newAuthContainer()
	wantErr := errors.New("postgres unavailable")

	var calls int32
	open := func() (*sqlstore.Container, error) {
		atomic.AddInt32(&calls, 1)
		return nil, wantErr
	}

	for i := 0; i < 10; i++ {
		container, err := holder.get(open)
		if container != nil {
			t.Fatalf("call %d: got non-nil container on error path", i)
		}
		if !errors.Is(err, wantErr) {
			t.Fatalf("call %d: got error %v, want %v", i, err, wantErr)
		}
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("open callback ran %d times, want exactly 1", got)
	}
	if holder.current() != nil {
		t.Fatalf("current() = %p, want nil after failed open", holder.current())
	}
}

// authPoolRecorder registra exatamente o que configureAuthPool aplicou.
type authPoolRecorder struct {
	maxOpen     int
	maxIdle     int
	maxIdleTime time.Duration
	maxLifetime time.Duration
}

func (r *authPoolRecorder) SetMaxOpenConns(n int)              { r.maxOpen = n }
func (r *authPoolRecorder) SetMaxIdleConns(n int)              { r.maxIdle = n }
func (r *authPoolRecorder) SetConnMaxIdleTime(d time.Duration) { r.maxIdleTime = d }
func (r *authPoolRecorder) SetConnMaxLifetime(d time.Duration) { r.maxLifetime = d }

// TestApplyAuthPoolSettingsApplyAllFour prova, de forma determinística, que os
// 4 valores configurados chegam ao pool. (SetConnMaxIdleTime/SetConnMaxLifetime
// não são legíveis de *sql.DB; por isso a aplicação é verificada num recorder e
// o efeito real em *sql.DB fica no teste seguinte.)
func TestApplyAuthPoolSettingsApplyAllFour(t *testing.T) {
	rec := &authPoolRecorder{}
	cfg := &config.Config{
		PostgresAuthMaxOpenConns:    10,
		PostgresAuthMaxIdleConns:    5,
		PostgresAuthConnMaxIdleTime: "1m",
		PostgresAuthConnMaxLifetime: "30m",
	}

	applyAuthPoolSettings(rec, cfg)

	if rec.maxOpen != 10 {
		t.Fatalf("SetMaxOpenConns = %d, want 10", rec.maxOpen)
	}
	if rec.maxIdle != 5 {
		t.Fatalf("SetMaxIdleConns = %d, want 5", rec.maxIdle)
	}
	if rec.maxIdleTime != time.Minute {
		t.Fatalf("SetConnMaxIdleTime = %v, want 1m", rec.maxIdleTime)
	}
	if rec.maxLifetime != 30*time.Minute {
		t.Fatalf("SetConnMaxLifetime = %v, want 30m", rec.maxLifetime)
	}
}

// TestConfigureAuthPoolAppliesToSQLDB cobre o efeito observável em *sql.DB:
// MaxOpenConnections é lido direto de Stats(); a duração é comprovada porque uma
// conexão mais velha que ConnMaxLifetime é descartada na reutilização e
// MaxLifetimeClosed sobe. (SetMaxIdleConns e SetConnMaxIdleTime não têm
// contrapartida legível em *sql.DB — por isso o teste anterior verifica os 4
// valores aplicados.)
func TestConfigureAuthPoolAppliesToSQLDB(t *testing.T) {
	t.Run("MaxOpenConnections", func(t *testing.T) {
		db, _, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock: %v", err)
		}
		defer db.Close()

		configureAuthPool(db, &config.Config{
			PostgresAuthMaxOpenConns:    7,
			PostgresAuthMaxIdleConns:    3,
			PostgresAuthConnMaxIdleTime: "1m",
			PostgresAuthConnMaxLifetime: "30m",
		})

		if got := db.Stats().MaxOpenConnections; got != 7 {
			t.Fatalf("Stats().MaxOpenConnections = %d, want 7", got)
		}
	})

	// sqlite real (o driver default de authDB) para exercitar o descarte por
	// lifetime: sqlmock não reabre a mesma conexão depois de fechada.
	t.Run("ConnMaxLifetime", func(t *testing.T) {
		dsn := fmt.Sprintf("file:%s/pool.db?mode=rwc", t.TempDir())
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		defer db.Close()

		configureAuthPool(db, &config.Config{
			PostgresAuthMaxOpenConns:    2,
			PostgresAuthMaxIdleConns:    1,
			PostgresAuthConnMaxIdleTime: "1m",
			PostgresAuthConnMaxLifetime: "1ms",
		})

		var n int
		if err := db.QueryRow("SELECT 1").Scan(&n); err != nil {
			t.Fatalf("first query: %v", err)
		}

		// Passa do ConnMaxLifetime (1ms) e consulta de novo: database/sql
		// descarta a conexão antiga e conta o fechamento.
		time.Sleep(20 * time.Millisecond)
		if err := db.QueryRow("SELECT 1").Scan(&n); err != nil {
			t.Fatalf("second query: %v", err)
		}
		if got := db.Stats().MaxLifetimeClosed; got == 0 {
			t.Fatal("Stats().MaxLifetimeClosed = 0, ConnMaxLifetime was not applied")
		}
	})
}

// newTestDeviceDB abre um *sql.DB sqlite + container migrado em diretório
// temporário, para os testes de close.
func newTestDeviceDB(t *testing.T) (*sql.DB, *sqlstore.Container) {
	t.Helper()
	dir := t.TempDir()
	dsn := fmt.Sprintf("file:%s/auth.db?_pragma=foreign_keys(1)&_busy_timeout=5000&mode=rwc&_journal_mode=WAL", dir)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	container := sqlstore.NewWithDB(db, "sqlite", nil)
	if err := container.Upgrade(context.Background()); err != nil {
		db.Close()
		t.Fatalf("upgrade container: %v", err)
	}
	return db, container
}

// TestAuthContainerCloseIsNilSafeAndIdempotent cobre holder vazio, duplo close
// e o efeito real: depois do close o *sql.DB responde erro de fechado.
func TestAuthContainerCloseIsNilSafeAndIdempotent(t *testing.T) {
	var nilHolder *authContainer
	if err := nilHolder.close(); err != nil {
		t.Fatalf("close on nil holder = %v, want nil", err)
	}
	if err := newAuthContainer().close(); err != nil {
		t.Fatalf("close on empty holder = %v, want nil", err)
	}
	if newAuthContainer().current() != nil {
		t.Fatal("current() on empty holder != nil")
	}

	db, container := newTestDeviceDB(t)
	holder := newAuthContainer()
	got, err := holder.get(func() (*sqlstore.Container, error) { return container, nil })
	if err != nil || got != container {
		t.Fatalf("get = (%p, %v), want (%p, nil)", got, err, container)
	}
	if holder.current() != container {
		t.Fatal("current() did not return the created container")
	}

	if err := holder.close(); err != nil {
		t.Fatalf("first close = %v, want nil", err)
	}
	if err := holder.close(); err != nil {
		t.Fatalf("second close = %v, want nil (idempotent)", err)
	}
	if err := db.Ping(); err == nil {
		t.Fatal("db.Ping() after close = nil, want 'database is closed'")
	} else if !errors.Is(err, sql.ErrConnDone) && err.Error() != "sql: database is closed" {
		t.Fatalf("db.Ping() after close = %v, want database closed error", err)
	}
}

// TestOpenDeviceContainerSQLite garante que o caminho sqlite (sem
// PostgresAuthDB) abre e migra o container usando o exPath do processo.
func TestOpenDeviceContainerSQLite(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "dbdata"), 0o755); err != nil {
		t.Fatalf("mkdir dbdata: %v", err)
	}

	w := whatsmeowService{config: &config.Config{PostgresAuthMaxOpenConns: 10}, exPath: dir}
	container, err := w.openDeviceContainer()
	if err != nil {
		t.Fatalf("openDeviceContainer: %v", err)
	}
	defer container.Close()

	if device := container.NewDevice(); device == nil {
		t.Fatal("NewDevice() = nil, container was not migrated/usable")
	}
}
