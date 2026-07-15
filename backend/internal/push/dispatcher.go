package push

import (
	"context"
	"encoding/json"
	"log"
	"strconv"

	"squirtlechat/internal/model"
	"squirtlechat/internal/service"
	"squirtlechat/internal/store"
	"squirtlechat/internal/ws"
	"squirtlechat/pkg/routing"

	"github.com/gin-gonic/gin"
)

// Dispatcher routes Kafka message events to local or remote gateway instances.
type Dispatcher struct {
	hub    *ws.Hub
	router *routing.Router
	msgs   *store.MessageStore
}

func NewDispatcher(hub *ws.Hub, router *routing.Router, msgs *store.MessageStore) *Dispatcher {
	return &Dispatcher{hub: hub, router: router, msgs: msgs}
}

func (d *Dispatcher) BroadcastRead(ctx context.Context, readerID int64, convID string, readSeq int64) {
	members, err := d.msgs.ListMemberReadSeqs(ctx, convID)
	if err != nil {
		return
	}
	payload, _ := json.Marshal(gin.H{
		"type": "read",
		"payload": gin.H{
			"conversation_id": convID,
			"user_id":         strconv.FormatInt(readerID, 10),
			"read_seq":        readSeq,
		},
	})
	for _, m := range members {
		if m.UserID == readerID {
			continue
		}
		d.pushToUser(ctx, m.UserID, "", "", 0, payload)
	}
}

func (d *Dispatcher) BroadcastTyping(ctx context.Context, convID string, fromUserID int64, typing bool, toUserIDs []int64) {
	payload, _ := json.Marshal(gin.H{
		"type": "typing",
		"payload": gin.H{
			"conversation_id": convID,
			"user_id":         strconv.FormatInt(fromUserID, 10),
			"typing":          typing,
		},
	})
	for _, uid := range toUserIDs {
		if uid == fromUserID {
			continue
		}
		d.pushToUser(ctx, uid, "", "", 0, payload)
	}
}

func (d *Dispatcher) BroadcastReaction(ctx context.Context, evt *service.ReactionEvent) {
	payload, _ := json.Marshal(gin.H{
		"type": "reaction",
		"payload": gin.H{
			"conversation_id": evt.ConversationID,
			"msg_id":          strconv.FormatInt(evt.MsgID, 10),
			"user_id":         strconv.FormatInt(evt.UserID, 10),
			"emoji":           evt.Emoji,
			"added":           evt.Added,
			"reactions":       evt.Summaries,
		},
	})
	targets := map[int64]struct{}{evt.UserID: {}}
	for _, uid := range evt.ToUserIDs {
		targets[uid] = struct{}{}
	}
	for uid := range targets {
		d.pushToUser(ctx, uid, "", "", 0, payload)
	}
}

func (d *Dispatcher) BroadcastPin(ctx context.Context, evt *service.PinEvent) {
	payload, _ := json.Marshal(gin.H{
		"type": "pin",
		"payload": gin.H{
			"conversation_id": evt.ConversationID,
			"msg_id":          strconv.FormatInt(evt.MsgID, 10),
			"user_id":         strconv.FormatInt(evt.UserID, 10),
			"pinned":          evt.Pinned,
			"pins":            evt.Pins,
		},
	})
	targets := map[int64]struct{}{evt.UserID: {}}
	for _, uid := range evt.ToUserIDs {
		targets[uid] = struct{}{}
	}
	for uid := range targets {
		d.pushToUser(ctx, uid, "", "", 0, payload)
	}
}

func (d *Dispatcher) BroadcastRecall(ctx context.Context, evt *store.RecallEvent) {
	payload, _ := json.Marshal(gin.H{
		"type": "recall",
		"payload": gin.H{
			"msg_id":          evt.MsgID,
			"client_msg_id":   evt.ClientMsgID,
			"conversation_id": evt.ConversationID,
			"seq":             evt.Seq,
			"msg_type":        model.MsgTypeSystem,
			"content":         "[已撤回]",
		},
	})
	targets := make(map[int64]struct{})
	for _, uid := range evt.ToUserIDs {
		targets[uid] = struct{}{}
	}
	targets[evt.FromUserID] = struct{}{}
	for uid := range targets {
		d.pushToUser(ctx, uid, "", "", 0, payload)
	}
}

func (d *Dispatcher) BroadcastEdit(ctx context.Context, evt *service.EditEvent) {
	if evt == nil || evt.Message == nil {
		return
	}
	m := evt.Message
	payload, _ := json.Marshal(gin.H{
		"type": "edit",
		"payload": gin.H{
			"msg_id":          strconv.FormatInt(m.ID, 10),
			"client_msg_id":   m.ClientMsgID,
			"conversation_id": m.ConversationID,
			"from_user_id":    strconv.FormatInt(m.FromUserID, 10),
			"seq":             m.Seq,
			"msg_type":        m.MsgType,
			"content":         m.Content,
			"edited_at":       m.EditedAt,
			"created_at":      m.CreatedAt,
		},
	})
	targets := map[int64]struct{}{m.FromUserID: {}}
	for _, uid := range evt.ToUserIDs {
		targets[uid] = struct{}{}
	}
	for uid := range targets {
		d.pushToUser(ctx, uid, "", "", 0, payload)
	}
}

func (d *Dispatcher) BroadcastPollVote(ctx context.Context, evt *service.PollVoteEvent) {
	if evt == nil {
		return
	}
	payload, _ := json.Marshal(gin.H{
		"type": "poll_vote",
		"payload": gin.H{
			"conversation_id": evt.ConversationID,
			"msg_id":          strconv.FormatInt(evt.MsgID, 10),
			"user_id":         strconv.FormatInt(evt.UserID, 10),
			"option_id":       evt.OptionID,
			"poll":            evt.Result,
		},
	})
	targets := map[int64]struct{}{evt.UserID: {}}
	for _, uid := range evt.ToUserIDs {
		targets[uid] = struct{}{}
	}
	for uid := range targets {
		d.pushToUser(ctx, uid, "", "", 0, payload)
	}
}

func (d *Dispatcher) BroadcastReminder(ctx context.Context, evt *service.ReminderEvent) {
	if evt == nil || evt.Reminder == nil {
		return
	}
	r := evt.Reminder
	payload, _ := json.Marshal(gin.H{
		"type": "reminder",
		"payload": gin.H{
			"id":              strconv.FormatInt(r.ID, 10),
			"conversation_id": r.ConversationID,
			"msg_id":          strconv.FormatInt(r.MsgID, 10),
			"preview":         r.Preview,
			"remind_at":       r.RemindAt,
		},
	})
	d.pushToUser(ctx, r.UserID, "", "", 0, payload)
}

func (d *Dispatcher) HandleEvent(ctx context.Context, evt *store.SentEvent) {
	payload, _ := json.Marshal(gin.H{
		"type": "message",
		"payload": gin.H{
			"msg_id":          evt.MsgID,
			"client_msg_id":   evt.ClientMsgID,
			"conversation_id": evt.ConversationID,
			"from_user_id":    evt.FromUserID,
			"seq":             evt.Seq,
			"msg_type":        evt.MsgType,
			"content":         evt.Content,
		},
	})

	targets := make(map[int64]struct{})
	for _, uid := range evt.ToUserIDs {
		if uid != evt.FromUserID {
			targets[uid] = struct{}{}
		}
	}
	targets[evt.FromUserID] = struct{}{}

	for uid := range targets {
		except := ""
		if uid == evt.FromUserID {
			except = evt.ExceptDevice
		}
		d.pushToUser(ctx, uid, except, "", evt.MsgID, payload)
	}
}

func (d *Dispatcher) HandleCrossPush(ctx context.Context, raw []byte) {
	var p routing.CrossPushPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		log.Printf("cross push decode: %v", err)
		return
	}
	if p.OnlyDevice != "" {
		d.hub.PushToDevice(p.UserID, p.OnlyDevice, p.Data)
		return
	}
	if p.ExceptDevice != "" {
		d.hub.PushToUserExceptDevice(p.UserID, p.ExceptDevice, p.Data)
		return
	}
	d.hub.PushToUser(p.UserID, p.Data)
}

func (d *Dispatcher) pushLocal(userID int64, exceptDevice, onlyDevice string, data []byte) {
	if onlyDevice != "" {
		d.hub.PushToDevice(userID, onlyDevice, data)
		return
	}
	if exceptDevice != "" {
		d.hub.PushToUserExceptDevice(userID, exceptDevice, data)
		return
	}
	d.hub.PushToUser(userID, data)
}

func (d *Dispatcher) pushLocalDevice(userID int64, deviceID, exceptDevice, onlyDevice string, data []byte) {
	if onlyDevice != "" && deviceID != onlyDevice {
		return
	}
	if exceptDevice != "" && deviceID == exceptDevice {
		return
	}
	d.hub.PushToDevice(userID, deviceID, data)
}

func (d *Dispatcher) pushToUser(ctx context.Context, userID int64, exceptDevice, onlyDevice string, msgID int64, data []byte) {
	routes, err := d.router.RoutesForUser(ctx, userID)
	if err != nil {
		log.Printf("routes user=%d: %v", userID, err)
	}
	if len(routes) == 0 {
		d.pushLocal(userID, exceptDevice, onlyDevice, data)
		if !d.hub.HasUser(userID) && d.msgs != nil && msgID > 0 {
			_ = d.msgs.AddOffline(ctx, userID, msgID)
		}
		return
	}

	remoteSent := map[string]bool{}
	for _, route := range routes {
		if onlyDevice != "" && route.DeviceID != onlyDevice {
			continue
		}
		if exceptDevice != "" && route.DeviceID == exceptDevice {
			continue
		}
		if route.InstanceID == d.router.InstanceID() {
			d.pushLocalDevice(userID, route.DeviceID, exceptDevice, onlyDevice, data)
			continue
		}
		if remoteSent[route.InstanceID] {
			continue
		}
		remoteSent[route.InstanceID] = true
		wrap, _ := json.Marshal(routing.CrossPushPayload{
			UserID:       userID,
			ExceptDevice: exceptDevice,
			OnlyDevice:   onlyDevice,
			Data:         json.RawMessage(data),
		})
		_ = d.router.PublishToInstance(ctx, route.InstanceID, wrap)
	}
}
