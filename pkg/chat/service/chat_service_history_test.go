package chat_service

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// A âncora do `POST /chat/history-sync` é o dado que localiza o bloco no
// aparelho principal. Um valor degenerado produz um pedido que o dispositivo
// não consegue casar — e a falha é silenciosa (HTTP 200 no ACK, nenhum evento
// `HistorySync`). Estes testes travam a validação e o clamp de `count`.
func TestBuildHistorySyncAnchor(t *testing.T) {
	chat, err := types.ParseJID("5527999999999@s.whatsapp.net")
	if err != nil {
		t.Fatalf("falha ao montar o JID de teste: %v", err)
	}
	ts := time.Date(2026, 10, 29, 15, 26, 4, 0, time.FixedZone("-03", -3*60*60))

	valid := func() *HistorySyncRequestStruct {
		return &HistorySyncRequestStruct{
			MessageInfo: &types.MessageInfo{
				MessageSource: types.MessageSource{Chat: chat},
				ID:            "3EB0B5C58763F48D6EE609",
				Timestamp:     ts,
			},
			Count: 50,
		}
	}

	t.Run("aceita âncora válida", func(t *testing.T) {
		info, count, err := buildHistorySyncAnchor(valid())
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if info.Chat != chat {
			t.Errorf("Chat = %v, quer %v", info.Chat, chat)
		}
		if info.ID != "3EB0B5C58763F48D6EE609" {
			t.Errorf("ID = %q", info.ID)
		}
		if !info.Timestamp.Equal(ts) {
			t.Errorf("Timestamp = %v, quer %v", info.Timestamp, ts)
		}
		if count != 50 {
			t.Errorf("count = %d, quer 50", count)
		}
	})

	t.Run("preserva IsFromMe e IsGroup", func(t *testing.T) {
		input := valid()
		input.MessageInfo.IsFromMe = true
		input.MessageInfo.IsGroup = true
		info, _, err := buildHistorySyncAnchor(input)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if !info.IsFromMe || !info.IsGroup {
			t.Errorf("IsFromMe/IsGroup perdidos: %+v", info.MessageSource)
		}
	})

	t.Run("count zero ou negativo vira o default", func(t *testing.T) {
		for _, input := range []int{0, -1} {
			data := valid()
			data.Count = input
			_, count, err := buildHistorySyncAnchor(data)
			if err != nil {
				t.Fatalf("count=%d: erro inesperado: %v", input, err)
			}
			if count != defaultHistorySyncCount {
				t.Errorf("count=%d virou %d, quer %d", input, count, defaultHistorySyncCount)
			}
		}
	})

	t.Run("count acima do teto é aparado", func(t *testing.T) {
		data := valid()
		data.Count = 5000
		_, count, err := buildHistorySyncAnchor(data)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if count != maxHistorySyncCount {
			t.Errorf("count = %d, quer %d", count, maxHistorySyncCount)
		}
	})

	t.Run("recusa pedido sem messageInfo", func(t *testing.T) {
		if _, _, err := buildHistorySyncAnchor(nil); err == nil {
			t.Error("esperava erro para data nil")
		}
		if _, _, err := buildHistorySyncAnchor(&HistorySyncRequestStruct{}); err == nil {
			t.Error("esperava erro para messageInfo nil")
		}
	})

	t.Run("recusa âncora degenerada", func(t *testing.T) {
		cases := map[string]func(*HistorySyncRequestStruct){
			"Chat vazio":     func(d *HistorySyncRequestStruct) { d.MessageInfo.Chat = types.JID{} },
			"ID vazio":       func(d *HistorySyncRequestStruct) { d.MessageInfo.ID = "" },
			"Timestamp zero": func(d *HistorySyncRequestStruct) { d.MessageInfo.Timestamp = time.Time{} },
		}
		for name, mutate := range cases {
			data := valid()
			mutate(data)
			if _, _, err := buildHistorySyncAnchor(data); err == nil {
				t.Errorf("%s: esperava erro", name)
			}
		}
	})
}

// Garante que o pedido montado é a peer message de HISTORY_SYNC_ON_DEMAND com
// a âncora correta — o tipo de mensagem que o aparelho principal responde com
// um evento `HistorySync`. Se alguém trocar `SendPeerMessage` por `SendMessage`
// no chat, este teste continua verde (ele cobre o payload), por isso o
// comentário no serviço é a trava do endereçamento; aqui travamos o conteúdo.
func TestHistorySyncRequestPayload(t *testing.T) {
	chat, err := types.ParseJID("5527999999999@s.whatsapp.net")
	if err != nil {
		t.Fatalf("falha ao montar o JID de teste: %v", err)
	}
	ts := time.Date(2026, 10, 29, 15, 26, 4, 0, time.FixedZone("-03", -3*60*60))

	info := types.MessageInfo{
		MessageSource: types.MessageSource{Chat: chat, IsFromMe: false},
		ID:            "3EB0B5C58763F48D6EE609",
		Timestamp:     ts,
	}

	msg := (&whatsmeow.Client{}).BuildHistorySyncRequest(&info, 50)
	protocol := msg.GetProtocolMessage()
	if protocol == nil {
		t.Fatal("mensagem sem ProtocolMessage")
	}
	if got := protocol.GetType(); got != waE2E.ProtocolMessage_PEER_DATA_OPERATION_REQUEST_MESSAGE {
		t.Errorf("ProtocolMessage.Type = %v, quer PEER_DATA_OPERATION_REQUEST_MESSAGE", got)
	}
	odr := protocol.GetPeerDataOperationRequestMessage().GetHistorySyncOnDemandRequest()
	if odr == nil {
		t.Fatal("sem HistorySyncOnDemandRequest")
	}
	if got := odr.GetChatJID(); got != chat.String() {
		t.Errorf("ChatJID = %q, quer %q", got, chat.String())
	}
	if got := odr.GetOldestMsgID(); got != info.ID {
		t.Errorf("OldestMsgID = %q, quer %q", got, info.ID)
	}
	if got := odr.GetOldestMsgFromMe(); got != info.IsFromMe {
		t.Errorf("OldestMsgFromMe = %v, quer %v", got, info.IsFromMe)
	}
	if got := odr.GetOnDemandMsgCount(); got != 50 {
		t.Errorf("OnDemandMsgCount = %d, quer 50", got)
	}
	// O campo diz "MS" mas o protocolo espera **segundos** (nota na lib).
	if got := odr.GetOldestMsgTimestampMS(); got != ts.Unix() {
		t.Errorf("OldestMsgTimestampMS = %d, quer %d (segundos)", got, ts.Unix())
	}
}
