package whatsmeow_service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"go.mau.fi/whatsmeow/store/sqlstore"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
)

// newTestLoggerManager cria um LoggerManager apontando para um diretório
// temporário e devolve também o diretório, para que o teste possa inspecionar o
// log da instância.
func newTestLoggerManager(t *testing.T) (*logger_wrapper.LoggerManager, string) {
	t.Helper()

	logDir := t.TempDir()
	return logger_wrapper.NewLoggerManager(&config.Config{LogDirectory: logDir}), logDir
}

// instanceLog lê o log da instância (vazio se nada foi escrito ainda).
func instanceLog(t *testing.T, logDir, instanceID string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(logDir, instanceID, "instance.log"))
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read instance log: %v", err)
	}
	return string(content)
}

func TestCloseContainerNilIsNoop(t *testing.T) {
	loggerManager, _ := newTestLoggerManager(t)

	// Não deve entrar em pânico nem logar nada.
	closeContainer("instancia-sem-container", nil, loggerManager)
}

func TestCloseContainerClosesUnderlyingDB(t *testing.T) {
	loggerManager, logDir := newTestLoggerManager(t)

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sqlmock db: %v", err)
	}

	// sqlstore.NewWithDB não abre conexão nem roda upgrade: apenas embrulha o
	// *sql.DB, que é exatamente o que queremos comprovar sendo fechado.
	container := sqlstore.NewWithDB(sqlDB, "sqlite", nil)

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("expected db to be usable before close, got: %v", err)
	}

	mock.ExpectClose()

	closeContainer("instancia-valida", container, loggerManager)

	if err := sqlDB.Ping(); err == nil {
		t.Fatal("expected underlying *sql.DB to be closed after closeContainer, but Ping succeeded")
	} else if !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected database closed error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected sqlmock expectations: %v", err)
	}

	if logContent := instanceLog(t, logDir, "instancia-valida"); logContent != "" {
		t.Fatalf("expected no error log on successful close, got: %s", logContent)
	}
}

func TestCloseContainerIsIdempotent(t *testing.T) {
	loggerManager, logDir := newTestLoggerManager(t)

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sqlmock db: %v", err)
	}

	container := sqlstore.NewWithDB(sqlDB, "sqlite", nil)

	mock.ExpectClose()

	// Simula `defer closeContainer(...)` + o fechamento explícito antes do
	// restart acontecendo os dois: o segundo Close não pode falhar nem logar.
	closeContainer("instancia-idempotente", container, loggerManager)
	closeContainer("instancia-idempotente", container, loggerManager)

	if err := sqlDB.Ping(); err == nil {
		t.Fatal("expected underlying *sql.DB to remain closed")
	}

	// `(*sql.DB).Close` devolve nil quando o pool já está fechado — é o que
	// permite o `defer` e o fechamento explícito coexistirem sem ruído.
	if err := container.Close(); err != nil {
		t.Fatalf("expected Close on an already-closed container to return nil, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected sqlmock expectations: %v", err)
	}

	if logContent := instanceLog(t, logDir, "instancia-idempotente"); logContent != "" {
		t.Fatalf("expected no error log on idempotent close, got: %s", logContent)
	}
}
