---
status: done
feature: connection-name
base: 0.7.3-beta
target: 0.7.4-beta
started: 2026-11-03
finished: 2026-11-03
---

# Progress — `CONNECTION_NAME` e `PLATFORM_TYPE`

## Resumo

Foram adicionadas ao fork duas envs opcionais para controlar o rótulo exibido em
WhatsApp → Aparelhos conectados: `CONNECTION_NAME` (nome) e `PLATFORM_TYPE` (tipo do
dispositivo). O default preserva o comportamento de `0.7.3-beta` (`chrome` + `OS_NAME`).

## Status das tarefas

| ID | Tarefa | Status |
|---|---|---|
| CN-01 | `ParsePlatformType`/`PlatformTypeNames` em `pkg/utils` | ✅ |
| CN-02 | Envs e validação no `pkg/config` | ✅ |
| CN-03 | Consumo no `StartClient` | ✅ |
| CN-04 | Documentação e `.env.example` | ✅ |
| CN-05 | Changelog e `VERSION` | ✅ |
| CN-06 | Verificação final e tag | parcial (build/vet/test ok; tag depende de autorização) |

## Linha do tempo

- 2026-11-03 — design aprovado (duas envs separadas, fatal no boot para valor inválido,
  escopo restrito ao fork). Worktree `feat/connection-name` criado a partir de `main` (`e9c4736`).
  Comportamento atual documentado: `PlatformType` hardcoded `chrome` em
  `pkg/whatsmeow/service/whatsmeow.go:367` e `DeviceProps.Os` vindo de `OS_NAME`.
- 2026-11-03 — CN-01/CN-02/CN-03 implementados via sub-agente. Revisão do agente principal
  trocou a variável local `os` por `displayName` (sombreava o pacote `os`).
- 2026-11-03 — CN-04/CN-05: docs do fork, `.env.example`, `CHANGELOG.md` e `VERSION`.
  A entrada foi promovida para `## v0.7.4-beta` porque o workflow de release extrai as notas
  por esse heading e o `Makefile` lê a versão dele.
- 2026-11-03 — verificação: `gofmt -l` limpo nos arquivos alterados, `go build ./...`,
  `go vet ./...` e `go test ./pkg/utils/... ./pkg/whatsmeow/...` verdes.

## TPP (pendências de pré-requisito)

- **Validação com aparelho real** de que `PLATFORM_TYPE=desktop` exibe **só** o nome em
  Aparelhos conectados. Sem aparelho disponível, o critério fica aberto; o default `chrome`
  mantém o comportamento anterior e o rollback é uma env.

## Desvios

- `pkg/core/c0.go` já estava fora do `gofmt` na base (`0.7.3-beta`) — arquivo ofuscado,
  intocado. Não é regressão desta feature.
- `golangci-lint` não está instalado no ambiente, então `make lint` não rodou; a cobertura
  equivalente foi `go vet ./...`.
- `pkg/config` não tem teste automatizado (não existia e `Load()` chamaria `LogFatal`); a
  validação do valor inválido foi verificada por smoke de boot.
