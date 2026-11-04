package chat_service

import (
	"context"
	"errors"
	"time"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"
)

type ChatService interface {
	ChatPin(data *BodyStruct, instance *instance_model.Instance) (string, error)
	ChatUnpin(data *BodyStruct, instance *instance_model.Instance) (string, error)
	ChatArchive(data *BodyStruct, instance *instance_model.Instance) (string, error)
	ChatUnarchive(data *BodyStruct, instance *instance_model.Instance) (string, error)
	ChatMute(data *BodyStruct, instance *instance_model.Instance) (string, error)
	ChatUnmute(data *BodyStruct, instance *instance_model.Instance) (string, error)
	HistorySyncRequest(data *HistorySyncRequestStruct, instance *instance_model.Instance) (*whatsmeow.SendResponse, error)
}

type chatService struct {
	clientPointer    map[string]*whatsmeow.Client
	whatsmeowService whatsmeow_service.WhatsmeowService
	loggerWrapper    *logger_wrapper.LoggerManager
}

type BodyStruct struct {
	Chat string `json:"chat"`
}

type HistorySyncRequestStruct struct {
	MessageInfo *types.MessageInfo `json:"messageInfo"`
	Count       int                `json:"count"`
}

const (
	// defaultHistorySyncCount espelha a recomendação do whatsmeow (50 por bloco).
	defaultHistorySyncCount = 50
	// maxHistorySyncCount é o teto aceito; acima disso o pedido é aparado.
	maxHistorySyncCount = 100
)

func (c *chatService) ensureClientConnected(instanceId string) (*whatsmeow.Client, error) {
	client := c.clientPointer[instanceId]
	c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking client connection status - Client exists: %v", instanceId, client != nil)

	if client == nil {
		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] No client found, attempting to start new instance", instanceId)
		err := c.whatsmeowService.StartInstance(instanceId)
		if err != nil {
			c.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to start instance: %v", instanceId, err)
			return nil, errors.New("no active session found")
		}

		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Instance started, waiting 2 seconds...", instanceId)
		time.Sleep(2 * time.Second)

		client = c.clientPointer[instanceId]
		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking new client - Exists: %v, Connected: %v",
			instanceId,
			client != nil,
			client != nil && client.IsConnected())

		if client == nil || !client.IsConnected() {
			c.loggerWrapper.GetLogger(instanceId).LogError("[%s] New client validation failed - Exists: %v, Connected: %v",
				instanceId,
				client != nil,
				client != nil && client.IsConnected())
			return nil, errors.New("no active session found")
		}
	} else if !client.IsConnected() {
		c.loggerWrapper.GetLogger(instanceId).LogError("[%s] Existing client is disconnected - Connected status: %v",
			instanceId,
			client.IsConnected())
		return nil, errors.New("client disconnected")
	}

	c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Client successfully validated - Connected: %v", instanceId, client.IsConnected())
	return client, nil
}

func (c *chatService) ChatPin(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildPin(recipient, true))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error pin chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

func (c *chatService) ChatUnpin(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildPin(recipient, false))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error unpin chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

func (c *chatService) ChatArchive(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildArchive(recipient, true, time.Time{}, nil))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error archive chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

func (c *chatService) ChatUnarchive(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildArchive(recipient, false, time.Time{}, nil))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error unarchive chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

func (c *chatService) ChatMute(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildMute(recipient, true, 1*time.Hour))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error mute chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

func (c *chatService) ChatUnmute(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildMute(recipient, false, 0*time.Hour))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error unmute chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

// buildHistorySyncAnchor valida e normaliza a mensagem de referência do
// `POST /chat/history-sync`.
//
// É puro (sem cliente/IO) para ser testável: a âncora é o dado que localiza o
// bloco no aparelho principal, e um valor degenerado — `Chat` vazio, `ID`
// vazio ou `Timestamp` zero — faz o whatsmeow montar um pedido que o
// dispositivo não consegue casar, produzindo um 200 silencioso sem nenhum
// bloco. Melhor recusar cedo.
func buildHistorySyncAnchor(data *HistorySyncRequestStruct) (types.MessageInfo, int, error) {
	if data == nil || data.MessageInfo == nil {
		return types.MessageInfo{}, 0, errors.New("messageInfo is required")
	}

	messageInfo := types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:     data.MessageInfo.Chat,
			IsFromMe: data.MessageInfo.IsFromMe,
			IsGroup:  data.MessageInfo.IsGroup,
		},
		ID:        data.MessageInfo.ID,
		Timestamp: data.MessageInfo.Timestamp,
	}

	if messageInfo.Chat.IsEmpty() {
		return types.MessageInfo{}, 0, errors.New("messageInfo.Chat is required")
	}
	if messageInfo.ID == "" {
		return types.MessageInfo{}, 0, errors.New("messageInfo.ID is required")
	}
	if messageInfo.Timestamp.IsZero() {
		return types.MessageInfo{}, 0, errors.New("messageInfo.Timestamp is required")
	}

	// A lib recomenda 50 por bloco; limita a faixa útil em vez de aceitar 0
	// (nenhuma mensagem) ou valores que o aparelho pode recusar.
	count := data.Count
	if count <= 0 {
		count = defaultHistorySyncCount
	}
	if count > maxHistorySyncCount {
		count = maxHistorySyncCount
	}

	return messageInfo, count, nil
}

// HistorySyncRequest pede ao **aparelho principal** um bloco de mensagens
// anteriores à mensagem de referência (`messageInfo`).
//
// Detalhe que já quebrou em produção: o pedido é uma **peer message**
// (`ProtocolMessage.HISTORY_SYNC_ON_DEMAND`) e precisa ser endereçada ao
// **próprio dispositivo** (`cli.getOwnID()`), não ao chat do contato. Usar
// `SendMessage(ctx, messageInfo.Chat, msg, {Peer: true})` envia o nó
// `category: "peer"` com `to: <contato>` — o aparelho principal nunca recebe o
// pedido, nada responde e nenhum evento `HistorySync` é emitido (o HTTP ainda
// responde 200, porque é só o ACK do envio do nó). O helper da biblioteca,
// `client.SendPeerMessage`, faz o endereçamento correto; é ele que deve ser
// usado (ver o comentário de `BuildHistorySyncRequest` no whatsmeow).
//
// A resposta HTTP continua sendo apenas o ACK: o bloco chega depois pelo
// webhook `HistorySync` com `syncType = ON_DEMAND` (6).
func (c *chatService) HistorySyncRequest(data *HistorySyncRequestStruct, instance *instance_model.Instance) (*whatsmeow.SendResponse, error) {
	messageInfo, count, err := buildHistorySyncAnchor(data)
	if err != nil {
		return nil, err
	}

	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	histRequest := client.BuildHistorySyncRequest(&messageInfo, count)

	// `SendPeerMessage` (e não `SendMessage` no chat): ver o comentário acima.
	res, err := client.SendPeerMessage(context.Background(), histRequest)
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error history sync request: %v", instance.Id, err)
		return nil, err
	}

	c.loggerWrapper.GetLogger(instance.Id).LogInfo(
		"[%s] history sync request sent (count=%d, is_from_me=%t, is_group=%t, request_id=%s)",
		instance.Id,
		count,
		messageInfo.IsFromMe,
		messageInfo.IsGroup,
		res.ID,
	)

	return &res, nil
}

func NewChatService(
	clientPointer map[string]*whatsmeow.Client,
	whatsmeowService whatsmeow_service.WhatsmeowService,
	loggerWrapper *logger_wrapper.LoggerManager,
) ChatService {
	return &chatService{
		clientPointer:    clientPointer,
		whatsmeowService: whatsmeowService,
		loggerWrapper:    loggerWrapper,
	}
}
