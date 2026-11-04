# Evolution GO - Changelog

## Não lançado

### ⚠️ Pendência conhecida (não resolvida na 0.7.5-beta)

- **`history-sync` ainda não recebe bloco: `onDemandReady` não é anunciado.** A
  `0.7.5-beta` corrigiu o **endereçamento** do pedido (peer message vai ao próprio
  dispositivo), e o ACK passou a devolver `Chat` = o próprio JID. Mesmo assim o
  WhatsApp **não emite** o `HistorySync` de `syncType = 6` (`ON_DEMAND`): o
  `DeviceProps` desta lib declara

  ```go
  OnDemandReady:         nil,
  CompleteOnDemandReady: nil,
  ```

  em `store/clientpayload.go` (`DeviceProps_HistorySyncConfig`, campos 16 e 18):
  o companion anuncia ao servidor que **não suporta** histórico sob demanda, e o
  pedido é descartado **sem erro no socket e sem log**. O `HistorySync` de
  bootstrap (tipos 0–5) funciona normalmente — só o `ON_DEMAND` não chega.

  **Correção a fazer:** ligar as duas flags (`new(true)`) na lib ou por override
  no fork antes do `StartClient`. Como o `DeviceProps` vai no payload de
  **registro**, a mudança **só vale em um novo pareamento** (não é reenviado em
  reconexão) — exige reparear o número, com queda de atendimento, e há risco de o
  WhatsApp recusar o handshake. Por isso **não foi aplicada** ainda; decisão de
  retomar depois (registrado no produto em
  `crm-whats/docs/plans/inprogress/2026-11-04-historico-whatsapp/progress.md`).

## v0.7.5-beta

**Docker:** `evolution-go:0.7.5-beta`

### 🔧 Correções
- **`POST /chat/history-sync` não produzia nenhum histórico** — o pedido de
  histórico sob demanda (`ProtocolMessage.HISTORY_SYNC_ON_DEMAND`) era enviado com
  `client.SendMessage(ctx, messageInfo.Chat, msg, {Peer: true})`, ou seja,
  endereçado ao **contato**. Peer message precisa ir para o **próprio
  dispositivo** (`cli.getOwnID()`): o aparelho principal nunca recebia o pedido e
  nenhum evento `HistorySync` era emitido — enquanto isso a rota respondia `200`,
  porque é apenas o ACK do envio do nó. Agora usa `client.SendPeerMessage`, como
  o próprio comentário de `BuildHistorySyncRequest` no whatsmeow indica. Efeito
  visível: o client que consumia o endpoint ficava esperando um bloco que nunca
  chegava, sem erro nenhum.
- **Validação da âncora do histórico** — `messageInfo` nulo, `Chat` vazio, `ID`
  vazio ou `Timestamp` zero agora retornam erro explicitamente, em vez de montar
  um pedido que o aparelho não consegue casar e responder `200` sem bloco.
- **`count` do histórico** — ausente ou `<= 0` passa a usar **50** (recomendação
  da lib) e valores acima de **100** são aparados, em vez de repassar `0` (nenhuma
  mensagem) ou valores que o aparelho pode recusar.
- Log de envio do pedido passou a registrar `count`, `is_from_me`, `is_group` e o
  `request_id` do ACK — sem JID, sem telefone e sem conteúdo.

### 🧪 Testes
- `pkg/chat/service/chat_service_history_test.go`: validação/clamp da âncora e o
  payload montado (`ChatJID`, `OldestMsgID`, `OldestMsgFromMe`,
  `OnDemandMsgCount` e `OldestMsgTimestampMS` em **segundos**).

## v0.7.4-beta

**Docker:** `evolution-go:0.7.4-beta`

### 🆕 New Features
- **`CONNECTION_NAME` e `PLATFORM_TYPE`** — duas variáveis opcionais que controlam o rótulo
  exibido em **WhatsApp → Aparelhos conectados**. `CONNECTION_NAME` define o nome mostrado
  (antes, o `DeviceProps.Os` vinha sempre do `OS_NAME`) e `PLATFORM_TYPE` define o tipo de
  dispositivo anunciado no pareamento, que estava fixo em `chrome` no código. Com
  `PLATFORM_TYPE=desktop` + `CONNECTION_NAME=CRM Imobi` a lista mostra apenas **CRM Imobi**;
  com o padrão (`chrome` + `OS_NAME`) nada muda. Valor inválido em `PLATFORM_TYPE` impede o
  boot com a lista de valores aceitos (`unknown` é recusado de propósito: nesse valor o
  WhatsApp descarta o nome e mostra "Other device"). Ambas resolvidas em memória no
  `StartClient` — sem mudança de schema, e a env vence inclusive em instâncias já criadas.
  **TPP:** a confirmação visual com aparelho real de que `desktop` exibe só o nome não foi
  executada; o default `chrome` mantém o comportamento anterior. Documentado em
  `docs/wiki/fundamentos/configuration.md` e `docs/wiki/referencia/environment-variables.md`.
  Nota operacional: o rótulo só é enviado em **novos pareamentos** — instâncias já pareadas
  mantêm o valor antigo até desconectar e parear de novo.

## v0.7.3-beta

**Docker:** `evolution-go:0.7.3-beta`

### 🔧 Correções
- **whatsmeow atualizado para 2026-11-02** — a versão fixada na 0.7.2
  (commit `b572e5bcb92b`, de 30/06/2026) ficou 125 dias atrás do upstream. O
  WhatsApp subiu a versão mínima de cliente nesse intervalo e passou a recusar a
  conexão com **405 client outdated**, impedindo pareamento e reconexão das
  instâncias. Agora em `v0.0.0-20261102152715-54038813c47f`.
- **Go 1.26** — o whatsmeow novo exige `go >= 1.26.0`; o `Dockerfile` passou a
  usar `golang:1.26.10-alpine` no estágio de build.
- **Ajustes de API do whatsmeow** — `UpdateBlocklist` e `GetBlocklist` agora
  recebem um `dhash`; `SetStatusMessage` passou a receber `types.SetStatusInput`
  (novo endpoint mex). O formato de resposta de bloqueio mudou de
  `Modifications` para `Items` (com `LID`/`PN`/`Active`) — a documentação foi
  atualizada, mas integrações que leem `Modifications` precisam de ajuste.
- **Migração automática do banco de autenticação** — o schema do whatsmeow sobe
  de v14 para v16 (`companion_meta_nonce`, `wasa_root_secret_id`). Roda sozinho
  ao abrir o container; testado com banco SQLite v14 existente.

### 🔧 CI
- **Release passa a ser disparado por tag.** O workflow
  `publish_docker_image.yml` (build a cada push em `main`, sem gerar Release)
  foi removido e substituído por `release.yml`: push de tag `X.Y.Z` ou
  `X.Y.Z-sufixo` valida o `VERSION`, publica a imagem multi-arch e cria o
  GitHub Release a partir da seção correspondente deste CHANGELOG. Tags com
  sufixo (`-beta`) não movem a tag `latest` e o Release sai como pre-release.
  Especificação em `spec/spec-process-cicd-release.md`.
- **Makefile** — a versão lida do CHANGELOG agora vem do heading `## vX.Y.Z`,
  e não mais do primeiro `vN.N.N` encontrado no arquivo (que podia ser uma
  pseudo-versão de módulo citada no texto).

## v0.7.2

**Docker:** `evoapicloud/evolution-go:0.7.2`

### 🆕 New Features
- **Passkey (WebAuthn) pairing** — support for linking accounts that the WhatsApp
  server locks behind a **passkey** (the *Shortcake* / CRSC flow). When the
  server demands a passkey, whatsmeow's `PairPasskeyRequest` is surfaced through a
  new ceremony flow: the backend mints a short-lived ceremony token, and a bundled
  browser extension (`passkey-helper`) runs the WebAuthn assertion on the
  `web.whatsapp.com` origin and posts it back. Three public endpoints drive it:
  `GET /passkey-ceremony/{token}`, `POST .../response`, `POST .../confirm`. The
  manager detects the passkey stage and shows an "Abrir WhatsApp Web" button.
  Confirmation is always manual (never auto-confirm on `SkipHandoffUX`).
  Configure the public API base via **`PASSKEY_PUBLIC_URL`**. Full guide:
  `docs/wiki/guias-api/passkey-pairing.md`. Note: there is no headless bypass —
  the ceremony requires the account owner's real authenticator; the extension is
  web-only.
- **Headless license auto-activation** — set `EVOLUTION_OPERATOR_EMAIL` to the
  email used in your first manual license registration; on startup the service
  silently calls `/v1/register/auto` and skips the browser flow (falls back to the
  manual flow if the email isn't registered yet).
- **Button message media support** — additional media handling for interactive
  button messages.

### 🔧 Improvements / CI
- **Dropped the whatsmeow fork — now uses official `go.mau.fi/whatsmeow`.** The
  project previously vendored a fork (`whatsmeow-lib` submodule) to carry a
  PostgreSQL pool patch; upstream rejected that patch in favor of `NewWithDB`
  (app-side config). Removing the fork also pulled in upstream's native passkey
  support. Pinned to the commit that adds passkeys
  (`v0.0.0-20260630180629-b572e5bcb92b`). The `sync-releases` workflow no longer
  re-adds the submodule.
- **QR pairing consumes `events.QR` directly** instead of `GetQRChannel`. The QR
  channel auto-confirms passkey on `SkipHandoffUX` and disconnects the socket when
  codes run out — both break an in-flight passkey ceremony. Connecting without it
  keeps the socket alive for as long as pairing (QR or passkey) needs. QR rotation
  now pauses while a passkey ceremony is active.
- **Public sync fixes** — the release workflow drops the obsolete whatsmeow-lib
  step, targets `evolution-foundation/*`, and now ships the `passkey-helper`
  extension to the public repo.

### 🐛 Bug Fixes
- **`POST /instance/pair` returned an empty `PairingCode`** — the handler
  swallowed `PairPhone` errors and returned HTTP 200 with `PairingCode: ""`, and
  the client wasn't connected/awaiting-auth before `PairPhone`. Now starts the
  instance, waits for the websocket, and surfaces real errors (#21).
- **`GET /instance/status` returned 400 after a manual disconnect** — now returns
  200 with the disconnected status instead of erroring until a container restart
  (#20).

### 🏷️ Org rename
- Repository references updated from **EvolutionAPI** to **evolution-foundation**
  (module path, imports, GitHub URLs, submodule URLs).

## v0.7.1

**Docker:** `evoapicloud/evolution-go:0.7.1`

### 🆕 New Features
- **Test-send modal in Manager** — new modal in the embedded manager UI to test message sending directly from the panel, covering text, media and interactive message types. Useful for validating an instance right after pairing without leaving the manager.

### 🔧 Improvements / CI
- **whatsmeow-lib SHA now pinned in the public sync** — the `sync-releases` workflow previously re-cloned whatsmeow `main` on every run, so the SHA listed in the CHANGELOG could drift from what the public repos actually built against. The workflow now captures the SHA from the dev submodule and checks out that exact commit in the target, restoring release reproducibility.
- **Repository cleanup** — dropped tracked binaries (`evolution-go`, `build/server`), IDE config (`.idea/`) and scratch files (`DIFF-COMPLETO.txt`, `API-INTERACTIVE-DOCS.txt`, `carousel-sender.html`). Expanded `.gitignore` to prevent reincidence.

### 📝 Docs
- **Postman collection** — added `Set Proxy` request and multipart hints on `/send/media`; collection file renamed from `Evolution GO.postman_collection (2).json` to `Evolution GO.postman_collection.json`.
- **Interactive messages docs** — additional examples and corrections.

## v0.7.0

**Docker:** `evoapicloud/evolution-go:0.7.0`

### 🆕 New Features
- **Multi-platform interactive messages** — Buttons, lists and carousel working on Android, iOS and WhatsApp Web/Desktop
  - **SendButton**: removed `ViewOnceMessage` wrapper that blocked rendering on iOS and WhatsApp Web; `Footer` and `Header` are now conditional
  - **SendList**: migrated from `InteractiveMessage`/`NativeFlowMessage` to legacy `ListMessage` (native protobuf) for broad compatibility
  - **SendCarousel**: new endpoint `POST /send/carousel` with cards (image, text, footer, buttons) and automatic JPEG thumbnail generation for instant image loading
  - `whatsmeow-lib`: added `biz` node for `InteractiveMessage` and pinned `product_list` type on the `biz` node for `ListMessage`
- **Base64 media support on `/send/media`** — The `url` field on `POST /send/media` now also accepts base64-encoded media. When the value does not start with `http://` or `https://`, it is treated as base64 and decoded; reuses the existing `SendMediaFile` flow
- **WhatsApp status endpoints** — new `POST /send/status/text` and `POST /send/status/media` publish text/image/video status to `status@broadcast`. Media endpoint supports both JSON (with URL) and multipart/form-data (file upload). Thanks @Eduardo-gato (#15)
- **Webhook routing for GROUP / NEWSLETTER** — when the primary `MESSAGE` / `SEND_MESSAGE` / `READ_RECEIPT` subscription is absent, events from `@g.us` chats are forwarded to `GROUP` subscribers and events from `@newsletter` chats to `NEWSLETTER` subscribers. Thanks @oismaelash (#18)

### 🔧 Improvements
- **Proxy protocol** — new optional `protocol` field (and `PROXY_PROTOCOL` env) supporting `http`, `https`, `socks5`. Replaces the hardcoded SOCKS5 dialer with `client.SetProxyAddress`, fixing HTTP-proxy QR pairing (#12). Thanks @TBDevMaster (#13)
- **WhatsApp Web version cache** — `fetchWhatsAppWebVersion` now caches the result for 1 hour with a mutex instead of issuing one request per instance startup. Thanks @VitorS0uza (#24)
- **Manager flicker fix** — instance page no longer replaces the list with skeleton cards on every 5s polling cycle (`hasLoaded` flag). Thanks @TBDevMaster (#14), closes #11
- **`WEBHOOKFILES` → `WEBHOOK_FILES`** — `.env.example`, docker-compose and docs aligned with the env var the runtime actually reads. Thanks @VitorS0uza (#22)
- **Dependency cleanup** — removed unused `github.com/evolution-foundation/evo-gate` from `go.mod`
- **whatsmeow-lib** bumped to `0923702fb`
- **Telemetry removed** — dropped legacy `pkg/telemetry`

### 🐛 Bug Fixes
- **`/message/edit`** — was silently ignored because the edit payload used `Conversation` while the original message was sent as `ExtendedTextMessage`. WhatsApp requires matching types; now the edit uses `ExtendedTextMessage` and the response returns the actual server timestamp instead of the zero value. Closes #16
- **Sticker upload to S3/MinIO** — when `webp.Decode` or `png.Encode` failed, the whole media pipeline aborted and the sticker was lost from the webhook. Now we log a warning and keep the raw `.webp` bytes so the sticker still reaches the bucket. Closes #5
- **Multipart `/send/media`** — the binary-upload branch silently dropped `mentionAll`, `mentionedJid` and `quoted`. These fields now parse from the form (with `mentionedJid` accepting repeated or comma-separated values) and reach the send service. Closes #2

### ⚠️ Breaking changes
- **Proxy** — previously all proxies were forced through SOCKS5. If you run SOCKS5 on a non-standard port (anything outside 1080/2080/42000-43000), set `PROXY_PROTOCOL=socks5` in the env or pass `"protocol": "socks5"` in the proxy body explicitly — otherwise the new protocol inference will fall back to HTTP.

### 📝 Docs
- **README** — updated WhatsApp support number and issue templates
- **Interactive messages guide** — new `docs/wiki/guias-api/api-interactive.md`
- **Proxy docs** — environment variables, configuration guide and API reference updated with the new `protocol` field

## v0.6.1

### 🆕 New Features
- **Group invite info endpoint** — `GET /group/invite-info` to get group details from invite link
- **Enhanced media sending** — GIF playback, video stickers, and transparent sticker support

### 🐛 Bug Fixes
- **Admin revoke** — Allow deleting messages from others in groups (admin revoke)

### 🔧 Improvements
- **Version management** — Reads version from `VERSION` file with ldflags fallback
- **CORS global middleware** — Applied before all routes
- **Makefile compatibility** — Fixed `$(shell)` syntax for GNU Make 3.81 (macOS default)
- **CI/CD cleanup** — Removed `develop` branch trigger and `homolog` tag from Docker workflow
- **README updated** — New links, documentation, and hosting info

## v0.6.0

### 🆕 New Features
- **Version from VERSION file** — Reads version from `VERSION` file at startup instead of hardcoded value

### 🔧 Improvements
- **Makefile compatibility** — Fixed `$(shell)` syntax for GNU Make 3.81 (macOS default)

## v0.5.4

### 🔧 Improvements
- **Update whatsmeow lib**

## v0.5.3

**Docker:** `evoapicloud/evolution-go:0.5.3`

### 🔧 Improvements

- **Update context handling in service methods** 
  - Refactored multiple service methods across various packages to include `context.Background()` as the first argument in client calls. This change ensures that all client interactions are properly context-aware, allowing for better cancellation and timeout management.
  - Updated methods in `call_service.go`, `community_service.go`, `group_service.go`, `message_service.go`, `newsletter_service.go`, `send_service.go`, `user_service.go`, and `whatsmeow.go` to enhance consistency and reliability in handling requests.
  - This adjustment improves the overall robustness of the API by ensuring that all client calls can leverage context for better control over execution flow and resource management.

## v0.5.2

**Docker:** `evoapicloud/evolution-go:0.5.2`

### 🆕 New Features
- **SetProxy Endpoint**: New endpoint `POST /instance/proxy/{instanceId}` to configure proxy for instances
  - Support for proxy with/without authentication
  - Validation of required fields (host, port)
  - Automatic cache update via reconnection
  - Integrated Swagger documentation

### 🔧 Improvements
- **CheckUser Fallback Logic**: Implemented intelligent fallback logic
  - If `formatJid=true` returns `IsInWhatsapp=false`, automatically retries with `formatJid=false`
  - Significant improvement in valid user detection
  - Added `RemoteJID` field to use WhatsApp-validated JID
- **LID/WhatsApp JID Swap**: Automatic handling of special cases
  - When `Sender` comes as `@lid` and `SenderAlt` comes as `@s.whatsapp.net`
  - Automatic inversion: `Sender` and `Chat` receive `@s.whatsapp.net`, `SenderAlt` receives `@lid`
  - Detailed logs for tracking swaps

### 🐛 Bug Fixes
- **SendMessage**: Standardization of WhatsApp-validated `remoteJID` usage
- **User Validation**: Improvement in phone number validation and formatting

---

## v0.5.1

**Docker:** `evoapicloud/evolution-go:0.5.1`

### 🔧 Improvements
- **Instance Deletion**: Enhance instance deletion and media storage path resolution
- **Media Storage**: Improvements in media storage and path resolution

---

## v0.5.0

**Docker:** `evoapicloud/evolution-go:0.5.0`

### 🔧 Improvements
- **Media Storage**: Enhance media storage and logging in Whatsmeow event handling
- **Retry Logic**: Implement retry logic for client connection and message sending
- **Media Handling**: Enhance media handling in event processing

---

## v0.4.9

**Docker:** `evoapicloud/evolution-go:0.4.9`

### 🔧 Improvements
- **Connection Handling**: Add instance update test scenarios and improve connection handling
- **FormatJid Field**: Update FormatJid field to pointer type for better handling in message structures
- **Dependencies**: Update dependencies and fix presence handling in Whatsmeow integration

---

## v0.4.8

**Docker:** `evoapicloud/evolution-go:0.4.8`

### 🔧 Improvements
- **Audio Duration**: Improve audio duration parsing in convertAudioToOpusWithDuration function

---

## v0.4.7

**Docker:** `evoapicloud/evolution-go:0.4.7`

### 🔧 Improvements
- **Phone Number Formatting**: Improve phone number formatting and validation in user service
- **Brazilian/Portuguese Numbers**: Update Brazilian and Portuguese number formatting in utils

### 🆕 New Features
- **Media Handling**: Enhance media handling in event processing

---

## v0.4.6

**Docker:** `evoapicloud/evolution-go:0.4.6`

### 🆕 New Features
- **User Existence Check**: Add user existence check configuration and JID validation middleware

---

## v0.4.5

**Docker:** `evoapicloud/evolution-go:0.4.5`

### 🔧 Improvements
- **Dependencies**: Update dependencies and enhance audio conversion functionality

---

## v0.4.4

**Docker:** `evoapicloud/evolution-go:0.4.4`

### 🆕 New Features
- **CLAUDE.md**: Add CLAUDE.md for project documentation and enhance RabbitMQ connection handling

---

## v0.4.3

**Docker:** `evoapicloud/evolution-go:0.4.3`

### 🔧 Improvements
- **PostgreSQL Connection**: Fix in PostgreSQL connection configuration for session auth
  - Controlled configuration of pool, idle, etc.
  - Adjustment on top of whatsmeow lib
- **User Endpoints**: Fix in 'User Info' and 'Check User' endpoints
  - Now return with contact's LID information

---

## v0.3.0

### 🆕 New Features
- **Own Message Reactions**: Additional 'fromMe' parameter using Chat id
- **CreatedAt Field**: CreatedAt field added to instances table

---

## v0.2.0

### 🆕 New Features
- **Advanced Settings**: Advanced configurations in instance creation
  - `alwaysOnline` (still to be implemented)
  - `rejectCall` - Automatically reject calls
  - `msgRejectCall` - Call rejection message
  - `readMessages` - Automatically mark messages as read
  - `ignoreGroups` - Ignore group messages
  - `ignoreStatus` - Ignore status messages
- **Advanced Settings Routes**: New routes for get and update of advanced settings
- **QR Code Control**: `QRCODE_MAX_COUNT` variable to control how many QR codes to generate before timeout
- **AMQP Events**: `AMQP_SPECIFIC_EVENTS` variable to individually select which events to receive in RabbitMQ

### 🔧 Improvements
- **Reconnect Endpoint**: Fix in reconnect endpoint
- **Sender Info**: `Sender` and `SenderAlt` no longer come with session id, only the id

### 🐛 Bug Fixes
- **QR Code Generation**: Fix to not generate QR code automatically after disconnection or logout

---

## v0.1.0

### 🆕 Initial Features
- Base implementation of Evolution API in Go
- WhatsApp integration via whatsmeow
- Instance system
- Basic message sending endpoints
- Webhook support
- RabbitMQ and NATS integration
- Authentication system
- Swagger documentation

---

## 📋 Migration Notes

### v0.5.2
- The new `SetProxy` endpoint requires admin permissions (`AuthAdmin`)
- The `CheckUser` fallback logic is automatic and transparent
- LID/WhatsApp JID handling is automatic

### v0.4.3
- Check PostgreSQL connection settings if using postgres auth

### v0.2.0
- Review advanced settings configurations if necessary
- Configure `QRCODE_MAX_COUNT` if you want to limit QR codes
- Configure `AMQP_SPECIFIC_EVENTS` for specific RabbitMQ events

---

## 🔗 Useful Links

- **Docker Hub**: `evoapicloud/evolution-go`
- **Documentation**: Swagger available at `/swagger/`
- **GitHub**: [Evolution API Go](https://github.com/evolution-foundation/evolution-go)

---

## 🤝 Contributing

To contribute to the project:
1. Fork the repository
2. Create a branch for your feature
3. Commit your changes
4. Open a Pull Request

---

*Last updated: October 2025*

