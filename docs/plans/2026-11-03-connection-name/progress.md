---
status: inprogress
feature: connection-name
base: 0.7.3-beta
target: 0.7.4-beta
started: 2026-11-03
---

# Progress — `CONNECTION_NAME` e `PLATFORM_TYPE`

## Resumo

Adicionar duas envs opcionais ao fork para controlar o rótulo exibido em
WhatsApp → Aparelhos conectados: `CONNECTION_NAME` (nome) e `PLATFORM_TYPE` (tipo do
dispositivo). Default preserva o comportamento de `0.7.3-beta`.

## Status das tarefas

| ID | Tarefa | Status |
|---|---|---|
| CN-01 | `ParsePlatformType`/`PlatformTypeNames` em `pkg/utils` | pendente |
| CN-02 | Envs e validação no `pkg/config` | pendente |
| CN-03 | Consumo no `StartClient` | pendente |
| CN-04 | Documentação e `.env.example` | pendente |
| CN-05 | Changelog e `VERSION` | pendente |
| CN-06 | Verificação final e tag | pendente |

## Linha do tempo

- 2026-11-03 — design aprovado (duas envs separadas, fatal no boot para valor inválido,
  escopo restrito ao fork). Worktree `feat/connection-name` criado a partir de `main` (`e9c4736`).
  Comportamento atual documentado: `PlatformType` hardcoded `chrome` em
  `pkg/whatsmeow/service/whatsmeow.go:367` e `DeviceProps.Os` vindo de `OS_NAME`.

## TPP (pendências de pré-requisito)

- Validação com aparelho real de que `PLATFORM_TYPE=desktop` exibe **só** o nome. Sem aparelho
  disponível, o critério fica aberto; o default `chrome` mantém o comportamento atual.

## Desvios

- Nenhum até aqui.
