package whatsmeow_service

import (
	"go.mau.fi/whatsmeow/proto/waE2E"
)

// Política de mídia do webhook (env `WEBHOOK_SKIP_VIDEO`).
//
// Contexto: ao receber mídia com `WEBHOOK_FILES=true`, a engine baixa o arquivo
// inteiro para memória (`[]byte`) e depois codifica em base64 para caber no
// payload do webhook. A codificação base64 custa ~1,33x do tamanho do arquivo e
// o Go mantém as duas cópias vivas no heap durante a conversão — na prática ~3x
// o tamanho do vídeo. Um vídeo de 97 MB virou ~130 MB de string e derrubou a
// engine por OOMKill (limite de 768Mi) em 18/11/2026.
//
// `WEBHOOK_SKIP_VIDEO=true` evita o pico na origem (não baixa vídeo) e a
// defesa em profundidade (`StripVideoBase64`) garante que nenhum `base64` de
// vídeo escape no payload, mesmo que outro caminho venha a preencher o campo.
// O restante dos metadados da mensagem (id, chat, tipo, caption, fileLength,
// URL) continua sendo enviado normalmente — só a mídia pesada sai.

// WebhookMediaKind identifica a mídia que seria baixada para o payload.
type WebhookMediaKind string

const (
	WebhookMediaImage    WebhookMediaKind = "image"
	WebhookMediaAudio    WebhookMediaKind = "audio"
	WebhookMediaDocument WebhookMediaKind = "document"
	WebhookMediaVideo    WebhookMediaKind = "video"
	WebhookMediaSticker  WebhookMediaKind = "sticker"
)

// ShouldDownloadWebhookMedia decide se a mídia deve ser baixada e anexada ao
// payload do webhook. Única exceção hoje é o vídeo com a flag ligada; todas as
// outras combinações preservam o comportamento histórico (baixar).
func ShouldDownloadWebhookMedia(kind WebhookMediaKind, skipVideo bool) bool {
	if kind == WebhookMediaVideo && skipVideo {
		return false
	}
	return true
}

// payloadMediaKind espelha a ORDEM da cadeia de download do handler
// (`img → audio → document → video → sticker → associatedImg → …`). É essa
// ordem que decide de qual mídia veio o `base64` do payload: uma mensagem pode
// ter imagem principal E vídeo associado, e nesse caso o `base64` é da imagem.
func payloadMediaKind(msg *waE2E.Message) (WebhookMediaKind, bool) {
	if msg == nil {
		return "", false
	}
	if msg.GetImageMessage() != nil {
		return WebhookMediaImage, true
	}
	if msg.GetAudioMessage() != nil {
		return WebhookMediaAudio, true
	}
	if msg.GetDocumentMessage() != nil {
		return WebhookMediaDocument, true
	}
	if msg.GetVideoMessage() != nil {
		return WebhookMediaVideo, true
	}
	if msg.GetStickerMessage() != nil {
		return WebhookMediaSticker, true
	}
	child := msg.GetAssociatedChildMessage()
	if child == nil || child.GetMessage() == nil {
		return "", false
	}
	childMsg := child.GetMessage()
	if childMsg.GetImageMessage() != nil {
		return WebhookMediaImage, true
	}
	if childMsg.GetAudioMessage() != nil {
		return WebhookMediaAudio, true
	}
	if childMsg.GetDocumentMessage() != nil {
		return WebhookMediaDocument, true
	}
	if childMsg.GetVideoMessage() != nil {
		return WebhookMediaVideo, true
	}
	if childMsg.GetStickerMessage() != nil {
		return WebhookMediaSticker, true
	}
	return "", false
}

// StripVideoBase64 remove a chave `base64` do `data["Message"]` quando a flag
// está ligada e o `base64` do payload veio de vídeo. Devolve true se removeu.
//
// Defesa em profundidade: a decisão de não baixar já evita o campo, mas o
// usuário pediu explicitamente para "ignorar o valor do campo base64" — este
// ponto garante o invariante no último passo antes do marshal, cobrindo
// qualquer caminho futuro (ex.: mídia vinda de storage, reply com vídeo) que
// volte a preencher o campo.
//
// O critério é a mídia que a cadeia de download escolheria, não "a mensagem tem
// algum vídeo": uma imagem com vídeo associado tem `base64` da **imagem**, e
// apagá-lo tiraria mídia legítima do produto.
func StripVideoBase64(dataMap map[string]interface{}, msg *waE2E.Message, enabled bool) bool {
	if !enabled {
		return false
	}
	kind, ok := payloadMediaKind(msg)
	if !ok || ShouldDownloadWebhookMedia(kind, enabled) {
		// Sem mídia, ou mídia que continua sendo baixada: o `base64` do payload
		// não é de vídeo, então não há o que remover.
		return false
	}
	message, ok := dataMap["Message"].(map[string]interface{})
	if !ok {
		return false
	}
	if _, hasBase64 := message["base64"]; !hasBase64 {
		return false
	}
	delete(message, "base64")
	return true
}
