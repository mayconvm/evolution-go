package whatsmeow_service

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/gomessguii/logger"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/evolution-foundation/evolution-go/pkg/config"
)

// authContainer detém o *sqlstore.Container compartilhado pelo processo.
//
// É um ponteiro de propósito: os métodos de whatsmeowService usam receiver por
// valor (`func (w whatsmeowService)`), então cada StartClient opera sobre uma
// CÓPIA da struct. Um sync.Once como campo de valor reiniciaria a cada cópia e
// recriaria o pool — exatamente o vazamento que esta correção elimina. O que
// precisa ser compartilhado é o endereço, não o valor.
type authContainer struct {
	once      sync.Once
	container *sqlstore.Container
	err       error
}

// newAuthContainer devolve o holder vazio. O container é criado na primeira
// chamada de get, quando a configuração e o exPath já estão disponíveis.
func newAuthContainer() *authContainer {
	return &authContainer{}
}

// get cria o container uma única vez através de open e memoriza o resultado —
// inclusive o erro. Chamadas concorrentes esperam a primeira terminar (sync.Once)
// e recebem o mesmo ponteiro. Se a criação falhou, as chamadas seguintes devolvem
// o mesmo erro; não há nova tentativa por chamada, senão um Postgres fora do ar
// voltaria a multiplicar pools a cada restart de instância.
func (h *authContainer) get(open func() (*sqlstore.Container, error)) (*sqlstore.Container, error) {
	if h == nil {
		return nil, fmt.Errorf("auth container holder is nil")
	}
	h.once.Do(func() {
		h.container, h.err = open()
	})
	return h.container, h.err
}

// current devolve o container já criado, ou nil se get nunca rodou (ou falhou).
func (h *authContainer) current() *sqlstore.Container {
	if h == nil {
		return nil
	}
	return h.container
}

// close libera o pool do container. É nil-safe (holder ou container nil) e
// idempotente: (*sql.DB).Close devolve nil quando já está fechado.
func (h *authContainer) close() error {
	if h == nil || h.container == nil {
		return nil
	}
	return h.container.Close()
}

// openDeviceContainer abre o container whatsmeow reaproveitado pelo processo.
//
// Diferente de sqlstore.New, o pool é aberto aqui para que ConfigureAuthPool
// possa governá-lo: sem SetMaxOpenConns/SetConnMaxIdleTime valem os defaults do
// database/sql (conexões ilimitadas e ociosas que nunca expiram).
func (w whatsmeowService) openDeviceContainer() (*sqlstore.Container, error) {
	dialect := "sqlite"
	dsn := fmt.Sprintf("file:%s/dbdata/main.db?_pragma=foreign_keys(1)&_busy_timeout=5000&cache=shared&mode=rwc&_journal_mode=WAL", w.exPath)
	if w.config.PostgresAuthDB != "" {
		dialect = "postgres"
		dsn = w.config.PostgresAuthDB
	}

	var dbLog waLog.Logger
	if w.config.WaDebug != "" {
		dbLog = waLog.Stdout("Database", w.config.WaDebug, true)
	}

	db, err := sql.Open(dialect, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open device database: %w", err)
	}

	configureAuthPool(db, w.config)

	container := sqlstore.NewWithDB(db, dialect, dbLog)
	// NewWithDB não roda migrações (só sqlstore.New roda): o upgrade é obrigatório.
	if err := container.Upgrade(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to upgrade device database: %w", err)
	}

	logger.LogInfo("[WHATSMEOW] Container de device criado (dialect=%s)", dialect)
	return container, nil
}

// authPoolSetter é o subconjunto de *sql.DB que o tuning do pool usa. Existe
// para que a aplicação dos 4 valores seja verificável sem banco.
type authPoolSetter interface {
	SetMaxOpenConns(int)
	SetMaxIdleConns(int)
	SetConnMaxIdleTime(time.Duration)
	SetConnMaxLifetime(time.Duration)
}

// configureAuthPool aplica os limites do pool do container de device. Valores
// ausentes ou <= 0 são ignorados (deixam o default do database/sql ou o que já
// estiver configurado); duração inválida é ignorada só naquele campo, com warn —
// tuning de pool não pode impedir a engine de subir nem de criar o container.
func configureAuthPool(db *sql.DB, cfg *config.Config) {
	if db == nil || cfg == nil {
		return
	}
	applyAuthPoolSettings(db, cfg)
}

// applyAuthPoolSettings é o miolo de configureAuthPool. Separado para o teste
// observar exatamente os 4 valores aplicados (duas das quatro configurações não
// são legíveis de volta em *sql.DB).
func applyAuthPoolSettings(setter authPoolSetter, cfg *config.Config) {
	if setter == nil || cfg == nil {
		return
	}

	if cfg.PostgresAuthMaxOpenConns > 0 {
		setter.SetMaxOpenConns(cfg.PostgresAuthMaxOpenConns)
	}
	if cfg.PostgresAuthMaxIdleConns > 0 {
		setter.SetMaxIdleConns(cfg.PostgresAuthMaxIdleConns)
	}
	if d, err := time.ParseDuration(cfg.PostgresAuthConnMaxIdleTime); err == nil && d > 0 {
		setter.SetConnMaxIdleTime(d)
	} else if cfg.PostgresAuthConnMaxIdleTime != "" {
		logger.LogWarn("[WHATSMEOW] invalid POSTGRES_AUTH_CONN_MAX_IDLE_TIME=%q, keeping default", cfg.PostgresAuthConnMaxIdleTime)
	}
	if d, err := time.ParseDuration(cfg.PostgresAuthConnMaxLifetime); err == nil && d > 0 {
		setter.SetConnMaxLifetime(d)
	} else if cfg.PostgresAuthConnMaxLifetime != "" {
		logger.LogWarn("[WHATSMEOW] invalid POSTGRES_AUTH_CONN_MAX_LIFETIME=%q, keeping default", cfg.PostgresAuthConnMaxLifetime)
	}
}
