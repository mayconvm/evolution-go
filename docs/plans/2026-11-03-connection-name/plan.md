# Plano — `CONNECTION_NAME` e `PLATFORM_TYPE`

Base: `feat/connection-name` (a partir de `main` = `e9c4736`, `0.7.3-beta`)
Design: `docs/plans/2026-11-03-connection-name/design.md`

Ordem: utils (fonte única) → config (validação) → service (consumo) → docs → release.
CN-01 e CN-02 são sequenciais; CN-03 depende de CN-01; CN-04 em diante é independente.

---

## CN-01 — `ParsePlatformType` e `PlatformTypeNames` em `pkg/utils`

**Arquivos:** `pkg/utils/utils.go`, `pkg/utils/utils_test.go`

**Fazer:**
- `ParsePlatformType(name string) (waCompanionReg.DeviceProps_PlatformType, error)`:
  - normaliza com `strings.ToLower(strings.TrimSpace(name))`;
  - `""` → erro `platform type is empty`;
  - `unknown`/`Unknown` → erro (o WhatsApp descarta o nome nesse valor);
  - aceita os aliases de `WhatsAppGetUserAgent`, incluindo o typo legado `andorid-tablet` e o
    novo `android-tablet`;
  - erro cita `strings.Join(PlatformTypeNames(), ", ")`.
- `PlatformTypeNames() []string`: nomes aceitos, ordenados alfabeticamente, sem `unknown`.
- `WhatsAppGetUserAgent` passa a delegar em `ParsePlatformType` (mantendo a assinatura pública
  para não quebrar consumidores externos), descartando o erro e devolvendo `UNKNOWN`.

**Aceite:** `go test ./pkg/utils/...` verde; tabela cobre `chrome`/`CHROME`/` Desktop `,
`andorid-tablet`, `android-tablet`, `unknown`, `""`, `navegador`.

**Comando:** `make test` (ou `go test ./pkg/utils/...` durante o desenvolvimento).

---

## CN-02 — `CONNECTION_NAME` e `PLATFORM_TYPE` no `config`

**Arquivos:** `pkg/config/env/env.go`, `pkg/config/config.go`

**Fazer:**
- Constantes `CONNECTION_NAME = "CONNECTION_NAME"` e `PLATFORM_TYPE = "PLATFORM_TYPE"` em
  `pkg/config/env/env.go`, logo após `OS_NAME`.
- Campos `ConnectionName string` e `PlatformType string` em `Config`, junto de `OsName`.
- Em `Load()`:
  - `connectionName := strings.TrimSpace(os.Getenv(config_env.CONNECTION_NAME))`;
  - `platformType` com default `chrome` e validação via `utils.ParsePlatformType`, abortando
    com `logger.LogFatal` e mensagem que inclui a lista de valores aceitos;
  - preencher os dois campos no literal `&Config{...}`.
- Import de `github.com/evolution-foundation/evolution-go/pkg/utils` (sem ciclo: `utils` não
  importa `config`).

**Aceite:** boot com `PLATFORM_TYPE=navegador` sai com código não-zero e mensagem listando os
valores; boot sem as envs sobe igual a hoje.

**Comando:** `make build` e smoke manual de boot.

---

## CN-03 — Consumir as envs no `StartClient`

**Arquivos:** `pkg/whatsmeow/service/whatsmeow.go`

**Fazer:** substituir o bloco `platformID, ok := ...` + resolução do `Os` (linhas ~367-375) por:

```go
if platformType, err := utils.ParsePlatformType(w.config.PlatformType); err == nil {
    store.DeviceProps.PlatformType = platformType.Enum()
} else {
    w.loggerWrapper.GetLogger(cd.Instance.Id).LogWarn(
        "[%s] invalid PLATFORM_TYPE, keeping default: %v", cd.Instance.Id, err)
}

os := w.config.ConnectionName
if os == "" {
    if cd.Instance.OsName == "" {
        cd.Instance.OsName = utils.WhatsAppGetUserOS()
    }
    os = cd.Instance.OsName
}
store.DeviceProps.Os = &os
```

**Restrições:** não alterar `RequireFullSync`, versão do WhatsApp nem o resto do `StartClient`.
Não tocar em `Instance.OsName` quando `CONNECTION_NAME` estiver setada (é o SO, não o rótulo).

**Aceite:** com `CONNECTION_NAME=CRM Imobi` + `PLATFORM_TYPE=desktop`, log/inspeção mostram
`DeviceProps.Os = "CRM Imobi"` e `PlatformType = DESKTOP`; sem as envs, `chrome` + `Linux`.

**Comando:** `make build`, `make vet`.

---

## CN-04 — Documentação e exemplo de ambiente

**Arquivos:** `docs/wiki/referencia/environment-variables.md`,
`docs/wiki/fundamentos/configuration.md`, `docker/examples/.env.example`

**Fazer:**
- Referência: `CONNECTION_NAME` (padrão vazio) e `PLATFORM_TYPE` (padrão `chrome`, lista de
  valores) na tabela "Servidor".
- Configuration: seções das duas envs com a tabela de resultado
  (`chrome`+vazio → "Google Chrome (Linux)"; `chrome`+nome → "Google Chrome (nome)";
  `desktop`+nome → "nome") e a nota de que só vale para **novos pareamentos**.
- `.env.example`: as duas variáveis comentadas, com `CONNECTION_NAME=` vazio e
  `PLATFORM_TYPE=chrome`.

**Aceite:** docs citam os valores aceitos reais (comparar com `PlatformTypeNames()`).

---

## CN-05 — Changelog e versão

**Arquivos:** `CHANGELOG.md`, `VERSION`

**Fazer:**
- Entrada em `## Não lançado` → `### 🆕 New Features`: as duas envs, o efeito na tabela de
  Aparelhos conectados, o default preservado, o fatal no boot para valor inválido e a nota de
  que instâncias já pareadas mantêm o rótulo antigo.
- `VERSION` → `0.7.4-beta` (o workflow de release valida `VERSION` contra a tag).

**Aceite:** `VERSION` bate com a tag planejada; changelog no mesmo commit da mudança.

---

## CN-06 — Verificação final e tag

**Comandos:** `make fmt vet lint test` e depois `git tag -a 0.7.4-beta`.

**Aceite:**
- [ ] `make fmt vet lint test` verde.
- [ ] `git diff --stat 0.7.3-beta` restrito aos arquivos listados acima (nenhum arquivo
      gerado, nenhum `docs/docs.go`/swagger tocado).
- [ ] Tag anotada `0.7.4-beta` criada e publicada (`git push origin 0.7.4-beta`) → workflow
      de release publica `ghcr.io/mayconvm/evolution-go:0.7.4-beta`.
- [ ] **TPP**: parear um número com `PLATFORM_TYPE=desktop` + `CONNECTION_NAME=CRM Imobi` e
      confirmar no celular que a lista mostra só o nome. Sem aparelho disponível, registrar o
      TPP no changelog e manter a tag publicada (o default `chrome` protege quem não setar).

**Fora de escopo:** consumo no `crm-whats`; alinhamento do `compose.yaml` do produto;
`PairPhone`/passkey.
