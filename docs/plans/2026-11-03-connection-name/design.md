# Design — `CONNECTION_NAME` e `PLATFORM_TYPE` (rótulo em Aparelhos conectados)

Data: 2026-11-03
Status: aprovado
Base: `0.7.3-beta` (main `e9c4736`)
Alvo: `0.7.4-beta`

## Problema

Ao parear um número por QR, o WhatsApp mostra o vínculo em **Aparelhos conectados** com o
rótulo `Google Chrome (Linux)`. Quem opera o produto não consegue identificar de onde veio o
acesso, e o nome do produto não aparece em lugar nenhum.

Evidência no código (base `0.7.3-beta`):

- `pkg/whatsmeow/service/whatsmeow.go:367` fixa `DeviceProps.PlatformType` em `chrome`
  (`store.DeviceProps_PlatformType_value[strings.ToUpper("chrome")]`);
- `pkg/whatsmeow/service/whatsmeow.go:371-375` preenche `DeviceProps.Os` com
  `instance.OsName`, que nasce de `OS_NAME` (`pkg/instance/service/instance_service.go:185`)
  e cai em `utils.WhatsAppGetUserOS()` (`Linux`) quando vazio.

O rótulo exibido é exatamente `DeviceProps` no *registration payload* do pareamento
(`store/clientpayload.go`, whatsmeow): `PlatformType` escolhe o "tipo" (navegador → mostra o
nome do navegador; `DESKTOP` → mostra só o `Os`). Logo, não existe configuração possível hoje
que mostre apenas um nome próprio.

## Decisões

| Tema | Decisão |
|---|---|
| Escopo | Duas envs opcionais: `CONNECTION_NAME` (nome exibido) e `PLATFORM_TYPE` (tipo do dispositivo). |
| Mecanismo | `CONNECTION_NAME` vence `OS_NAME` na resolução de `DeviceProps.Os`; `PLATFORM_TYPE` vence o `chrome` hardcoded. Ambas resolvidas **em memória** no `StartClient`, sem tocar em schema nem em instâncias persistidas. |
| Valores de `PLATFORM_TYPE` | Os mesmos aliases já suportados por `utils.WhatsAppGetUserAgent` (`chrome`, `desktop`, `mac`, `android`, `android-phone`, `android-tablet`, `ios-phone`, `ios-catalyst`, `ipad`, `wearos`, `ie`, `edge`, `safari`, `firefox`, `opera`, `uwp`, `aloha`, `tv-tcl`). Case-insensitive. |
| Valor inválido | **Fatal no boot**, com a lista de valores aceitos. `UNKNOWN` é explicitamente recusado. |
| Default | `PLATFORM_TYPE` = `chrome` e `CONNECTION_NAME` vazio → comportamento idêntico ao de hoje. |
| Fora de escopo | Consumo no `crm-whats` (Helm/compose/spec 09) — feature separada. |

### Alternativas descartadas

1. **Só `OS_NAME` na aplicação consumidora, sem tocar no fork.** Zero código, mas produz
   `Google Chrome (CRM Imobi)` — o nome do produto aparece entre parênteses, atrás do nome de
   um navegador. Descartado: não resolve o problema de identificação.
2. **Um único env com o nome e `PlatformType` sempre `DESKTOP`.** Menos superfície, mas tiraria
   do operador a possibilidade de manter o comportamento atual de navegador (que é o que o
   WhatsApp valida melhor) e misturaria duas decisões independentes. Descartado pelo usuário.
3. **`WARN` e fallback para `chrome` em valor inválido.** Boot nunca quebra, mas o erro só
   aparece no celular como "Other device" (o WhatsApp **descarta** o `Os` quando o
   `PlatformType` é `UNKNOWN`) — falha silenciosa pior que a de hoje. Descartado.
4. **Override por instância (API `/instance/create`).** Exigiria alterar o contrato HTTP, o
   schema do fork e o produto. O rótulo é do *deployment*, não da instância. Descartado.

## Arquitetura

### `pkg/config/env/env.go`

```go
OS_NAME                 = "OS_NAME"
CONNECTION_NAME         = "CONNECTION_NAME"
PLATFORM_TYPE           = "PLATFORM_TYPE"
```

### `pkg/config/config.go`

Campos novos em `Config` (junto de `OsName`):

```go
ConnectionName string
PlatformType   string
```

Em `Load()`:

```go
connectionName := strings.TrimSpace(os.Getenv(config_env.CONNECTION_NAME))

platformType := strings.ToLower(strings.TrimSpace(os.Getenv(config_env.PLATFORM_TYPE)))
if platformType == "" {
    platformType = "chrome" // default histórico: mantém o comportamento atual
}
if _, err := utils.ParsePlatformType(platformType); err != nil {
    logger.LogFatal("[CONFIG] invalid PLATFORM_TYPE ...: %v", err)
}
```

`Config` passa a importar `pkg/utils`. `pkg/utils` **não** importa `pkg/config` (verificado),
então não há ciclo.

### `pkg/utils/utils.go`

Fonte única do mapeamento. `WhatsAppGetUserAgent` hoje é código morto (nenhum chamador);
passa a ser o motor de `ParsePlatformType`:

```go
// ParsePlatformType converte o valor de PLATFORM_TYPE no enum do whatsmeow.
// Aceita os aliases de WhatsAppGetUserAgent, case-insensitive. UNKNOWN é
// recusado: o WhatsApp descarta o nome exibido nesse caso.
func ParsePlatformType(name string) (waCompanionReg.DeviceProps_PlatformType, error)

// PlatformTypeNames lista os valores aceitos, ordenados, para a mensagem de erro.
func PlatformTypeNames() []string
```

Regras:

- `""` → erro (o default é aplicado por `config.Load`, não aqui);
- alias conhecido → enum correspondente;
- qualquer outro valor, inclusive `unknown` → erro cuja mensagem contém `PlatformTypeNames()`;
- `andorid-tablet` (typo histórico do switch) continua aceito como alias legado **e**
  `android-tablet` passa a ser aceito; não remover o typo para não quebrar configuração
  existente.

### `pkg/whatsmeow/service/whatsmeow.go` — `StartClient`

```go
// PlatformType e nome exibido vêm do deployment (CONNECTION_NAME/PLATFORM_TYPE),
// não da instância: o rótulo em "Aparelhos conectados" é único por engine.
if platformType, err := utils.ParsePlatformType(w.config.PlatformType); err == nil {
    store.DeviceProps.PlatformType = platformType.Enum()
} else {
    w.loggerWrapper.GetLogger(cd.Instance.Id).LogWarn("[%s] invalid PLATFORM_TYPE, keeping default: %v", cd.Instance.Id, err)
}

if w.config.ConnectionName != "" {
    store.DeviceProps.Os = &w.config.ConnectionName
} else {
    if cd.Instance.OsName == "" {
        cd.Instance.OsName = utils.WhatsAppGetUserOS()
    }
    store.DeviceProps.Os = &cd.Instance.OsName
}
```

O `LogWarn` é defesa em profundidade: `Load()` já aborta com valor inválido, mas o service
não pode depender de ordem de inicialização para não enviar `UNKNOWN` ao WhatsApp.

Comportamento resultante:

| `PLATFORM_TYPE` | `CONNECTION_NAME` | Aparelhos conectados |
|---|---|---|
| `chrome` (default) | vazio | Google Chrome (Linux) — **igual a hoje** |
| `chrome` | `CRM Imobi` | Google Chrome (CRM Imobi) |
| `desktop` | `CRM Imobi` | **CRM Imobi** |

### Documentação

- `docs/wiki/referencia/environment-variables.md`: linhas em "Servidor", com padrão e os
  valores aceitos de `PLATFORM_TYPE`.
- `docs/wiki/fundamentos/configuration.md`: seções `CONNECTION_NAME` e `PLATFORM_TYPE` com a
  tabela de resultado acima e a nota de que o valor só vale para **novos pareamentos**.
- `docker/examples/.env.example`: as duas variáveis comentadas.
- `CHANGELOG.md`: entrada em "Não lançado".

### Nota operacional (documentada, não é bug)

`DeviceProps` só vai no *registration payload* do pareamento. Instâncias **já pareadas**
continuam exibindo o rótulo antigo até desconectar e gerar novo QR — e desconectar uma
instância em produção interrompe o atendimento até o repareamento.

## Fora de escopo

- Consumo no `crm-whats` (Helm `values.yaml`, `compose.yaml`, spec 09 §3). Decisão do usuário:
  só o fork nesta rodada.
- Alinhar `compose.yaml` do `crm-whats` (upstream `0.7.2-beta`) ao fork. Fica como está.
- Lock de concorrência para `store.DeviceProps` (estado global do whatsmeow, já escrito sem
  lock antes desta mudança; hoje o valor é o mesmo para todas as instâncias).
- Passkey/`PairPhone`: `instance_service.go:538` passa `"Chrome (Linux)"` fixo como
  `clientDisplayName`. É outro fluxo (código de pareamento, não QR) e o nome lá é validado pelo
  servidor; não muda aqui.

## Critérios de aceite

- [ ] Sem `CONNECTION_NAME`/`PLATFORM_TYPE`, o comportamento é byte a byte o de `0.7.3-beta`
      (`chrome` + `OS_NAME`/`Linux`).
- [ ] `PLATFORM_TYPE=desktop` e `CONNECTION_NAME=CRM Imobi` resultam em
      `DeviceProps.PlatformType = DESKTOP` e `DeviceProps.Os = "CRM Imobi"`.
- [ ] `PLATFORM_TYPE` inválido (ex.: `navegador`) aborta o boot com mensagem que lista os
      valores aceitos; `Load()` não retorna config inválida.
- [ ] `PLATFORM_TYPE` é case-insensitive (`DESKTOP`, `Desktop`, `desktop`).
- [ ] `ParsePlatformType` coberto por teste de tabela (aceitos, aliases legados, inválidos,
      vazio) passando em `go test ./pkg/utils/...`.
- [ ] `make fmt vet lint test` verde no fork.
- [ ] `CHANGELOG.md` com a entrada e `VERSION` em `0.7.4-beta`.
- [ ] Tag anotada `0.7.4-beta` publicada (workflow de release do fork → GHCR).
- [ ] **TPP**: validação com aparelho real de que `desktop` mostra só o nome. Sem aparelho, o
      critério fica aberto e registrado como TPP no changelog.
