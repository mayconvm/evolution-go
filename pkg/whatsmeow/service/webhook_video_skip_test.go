package whatsmeow_service

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

// TestShouldDownloadWebhookMedia cobre a matriz da política `WEBHOOK_SKIP_VIDEO`:
// com a flag desligada (default, env ausente) tudo é baixado — comportamento
// histórico; com a flag ligada só o vídeo deixa de ser baixado.
func TestShouldDownloadWebhookMedia(t *testing.T) {
	allKinds := []WebhookMediaKind{
		WebhookMediaImage,
		WebhookMediaAudio,
		WebhookMediaDocument,
		WebhookMediaVideo,
		WebhookMediaSticker,
	}

	t.Run("flag ausente mantém o comportamento atual", func(t *testing.T) {
		for _, kind := range allKinds {
			if !ShouldDownloadWebhookMedia(kind, false) {
				t.Fatalf("kind %q deveria ser baixado com a flag desligada", kind)
			}
		}
	})

	t.Run("flag ligada só suprime vídeo", func(t *testing.T) {
		for _, kind := range allKinds {
			got := ShouldDownloadWebhookMedia(kind, true)
			if kind == WebhookMediaVideo {
				if got {
					t.Fatalf("vídeo NÃO deveria ser baixado com WEBHOOK_SKIP_VIDEO=true")
				}
				continue
			}
			if !got {
				t.Fatalf("kind %q deveria continuar sendo baixado com WEBHOOK_SKIP_VIDEO=true", kind)
			}
		}
	})
}

func TestPayloadMediaKind(t *testing.T) {
	cases := []struct {
		name   string
		msg    *waE2E.Message
		want   WebhookMediaKind
		wantOk bool
	}{
		{name: "nil", msg: nil, want: "", wantOk: false},
		{name: "texto", msg: &waE2E.Message{Conversation: stringPtr("oi")}, want: "", wantOk: false},
		{name: "vídeo", msg: &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}, want: WebhookMediaVideo, wantOk: true},
		{name: "imagem", msg: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}, want: WebhookMediaImage, wantOk: true},
		{name: "áudio", msg: &waE2E.Message{AudioMessage: &waE2E.AudioMessage{}}, want: WebhookMediaAudio, wantOk: true},
		{name: "documento", msg: &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{}}, want: WebhookMediaDocument, wantOk: true},
		{name: "sticker", msg: &waE2E.Message{StickerMessage: &waE2E.StickerMessage{}}, want: WebhookMediaSticker, wantOk: true},
		{
			name:   "vídeo em associatedChildMessage",
			msg:    &waE2E.Message{AssociatedChildMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}}},
			want:   WebhookMediaVideo,
			wantOk: true,
		},
		{
			name:   "imagem em associatedChildMessage",
			msg:    &waE2E.Message{AssociatedChildMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}}},
			want:   WebhookMediaImage,
			wantOk: true,
		},
		{
			// A imagem principal vence a ordem da cadeia: é ela que seria baixada.
			name: "imagem principal com vídeo associado escolhe a imagem",
			msg: &waE2E.Message{
				ImageMessage:           &waE2E.ImageMessage{},
				AssociatedChildMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}},
			},
			want:   WebhookMediaImage,
			wantOk: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := payloadMediaKind(tc.msg)
			if ok != tc.wantOk || got != tc.want {
				t.Fatalf("payloadMediaKind() = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.wantOk)
			}
		})
	}
}

// TestStripVideoBase64 garante o invariante pedido pelo usuário ("ignore o
// valor do campo base64"): com a flag ligada, o campo base64 de vídeo nunca
// sobrevive ao payload; o resto dos metadados fica intacto.
func TestStripVideoBase64(t *testing.T) {
	t.Run("flag desligada preserva o base64 atual", func(t *testing.T) {
		dataMap := map[string]interface{}{
			"Message": map[string]interface{}{"base64": "AAA", "caption": "vídeo"},
		}
		if StripVideoBase64(dataMap, &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}, false) {
			t.Fatal("não deveria remover base64 com a flag desligada")
		}
		message := dataMap["Message"].(map[string]interface{})
		if message["base64"] != "AAA" {
			t.Fatal("base64 deveria continuar presente com a flag desligada")
		}
	})

	t.Run("flag ligada remove base64 de vídeo e mantém metadados", func(t *testing.T) {
		fileLength := uint64(101_711_872) // ~97 MB
		dataMap := map[string]interface{}{
			"Message": map[string]interface{}{
				"base64":  "AAA",
				"caption": "tour da casa",
				"videoMessage": map[string]interface{}{
					"fileLength": float64(fileLength),
				},
			},
			"ID":   "3EB0ABC",
			"Chat": "5511999999999@s.whatsapp.net",
		}

		msg := &waE2E.Message{
			VideoMessage: &waE2E.VideoMessage{
				Caption:    stringPtr("tour da casa"),
				FileLength: &fileLength,
			},
		}

		if !StripVideoBase64(dataMap, msg, true) {
			t.Fatal("deveria ter removido o base64 de vídeo")
		}

		message := dataMap["Message"].(map[string]interface{})
		if _, hasBase64 := message["base64"]; hasBase64 {
			t.Fatal("base64 não deveria existir no payload com WEBHOOK_SKIP_VIDEO=true")
		}
		if message["caption"] != "tour da casa" {
			t.Fatal("caption do vídeo deveria continuar no payload")
		}
		if dataMap["ID"] != "3EB0ABC" || dataMap["Chat"] != "5511999999999@s.whatsapp.net" {
			t.Fatal("metadados da mensagem deveriam continuar no payload")
		}
	})

	t.Run("flag ligada não toca em base64 de imagem", func(t *testing.T) {
		dataMap := map[string]interface{}{
			"Message": map[string]interface{}{"base64": "IMG"},
		}
		if StripVideoBase64(dataMap, &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}, true) {
			t.Fatal("não deveria remover base64 de imagem")
		}
		message := dataMap["Message"].(map[string]interface{})
		if message["base64"] != "IMG" {
			t.Fatal("base64 de imagem deveria continuar presente")
		}
	})

	// Uma mensagem pode ter imagem principal E vídeo em `associatedChildMessage`.
	// A cadeia de download escolhe a imagem primeiro, então o `base64` do payload
	// é dela: apagar seria tirar mídia legítima do produto.
	t.Run("imagem com vídeo associado preserva o base64 da imagem", func(t *testing.T) {
		dataMap := map[string]interface{}{
			"Message": map[string]interface{}{"base64": "IMG"},
		}
		msg := &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{},
			AssociatedChildMessage: &waE2E.FutureProofMessage{
				Message: &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}},
			},
		}
		if StripVideoBase64(dataMap, msg, true) {
			t.Fatal("o base64 é da imagem e deveria continuar")
		}
	})

	t.Run("vídeo em associatedChildMessage sem mídia principal é removido", func(t *testing.T) {
		dataMap := map[string]interface{}{
			"Message": map[string]interface{}{"base64": "VID"},
		}
		msg := &waE2E.Message{
			AssociatedChildMessage: &waE2E.FutureProofMessage{
				Message: &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}},
			},
		}
		if !StripVideoBase64(dataMap, msg, true) {
			t.Fatal("base64 de vídeo associado deveria ser removido")
		}
		message := dataMap["Message"].(map[string]interface{})
		if _, hasBase64 := message["base64"]; hasBase64 {
			t.Fatal("base64 não deveria existir no payload")
		}
	})

	t.Run("mensagem sem Message não quebra", func(t *testing.T) {
		dataMap := map[string]interface{}{}
		if StripVideoBase64(dataMap, &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}, true) {
			t.Fatal("sem Message não há o que remover")
		}
	})
}
