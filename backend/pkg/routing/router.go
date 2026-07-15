package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	onlineKeyPrefix = "online:"
	pushChanPrefix  = "ws:push:"
	onlineTTL       = 2 * time.Minute
)

// Router tracks WS connections across gateway instances via Redis.
type Router struct {
	rdb        *goredis.Client
	instanceID string
}

func New(rdb *goredis.Client, instanceID string) *Router {
	return &Router{rdb: rdb, instanceID: instanceID}
}

func (r *Router) InstanceID() string { return r.instanceID }

func onlineKey(userID int64, deviceID string) string {
	return fmt.Sprintf("%s%d:%s", onlineKeyPrefix, userID, deviceID)
}

func pushChannel(instanceID string) string {
	return pushChanPrefix + instanceID
}

// Register marks user device online on this instance.
func (r *Router) Register(ctx context.Context, userID int64, deviceID string) error {
	return r.rdb.Set(ctx, onlineKey(userID, deviceID), r.instanceID, onlineTTL).Err()
}

// Refresh extends TTL for active connection.
func (r *Router) Refresh(ctx context.Context, userID int64, deviceID string) error {
	return r.rdb.Expire(ctx, onlineKey(userID, deviceID), onlineTTL).Err()
}

// Unregister removes user device route.
func (r *Router) Unregister(ctx context.Context, userID int64, deviceID string) error {
	key := onlineKey(userID, deviceID)
	val, err := r.rdb.Get(ctx, key).Result()
	if err == goredis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	if val == r.instanceID {
		return r.rdb.Del(ctx, key).Err()
	}
	return nil
}

type DeviceRoute struct {
	DeviceID   string
	InstanceID string
}

// RoutesForUser returns all online device routes for a user.
func (r *Router) RoutesForUser(ctx context.Context, userID int64) ([]DeviceRoute, error) {
	pattern := fmt.Sprintf("%s%d:*", onlineKeyPrefix, userID)
	var routes []DeviceRoute
	iter := r.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		inst, err := r.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		// key: online:{userID}:{deviceID}
		var uid int64
		var deviceID string
		if _, err := fmt.Sscanf(key, onlineKeyPrefix+"%d:%s", &uid, &deviceID); err != nil {
			continue
		}
		routes = append(routes, DeviceRoute{DeviceID: deviceID, InstanceID: inst})
	}
	return routes, iter.Err()
}

// IsUserOnline reports whether any device for the user is currently online.
func (r *Router) IsUserOnline(ctx context.Context, userID int64) (bool, error) {
	routes, err := r.RoutesForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return len(routes) > 0, nil
}

// BatchOnline returns online flags for the given user IDs.
func (r *Router) BatchOnline(ctx context.Context, userIDs []int64) (map[int64]bool, error) {
	out := make(map[int64]bool, len(userIDs))
	for _, id := range userIDs {
		on, err := r.IsUserOnline(ctx, id)
		if err != nil {
			return nil, err
		}
		out[id] = on
	}
	return out, nil
}

// PublishToInstance sends push payload to target gateway via Redis pub/sub.
func (r *Router) PublishToInstance(ctx context.Context, instanceID string, payload []byte) error {
	if instanceID == r.instanceID {
		return nil
	}
	return r.rdb.Publish(ctx, pushChannel(instanceID), payload).Err()
}

// Subscribe listens for cross-instance push on this gateway.
func (r *Router) Subscribe(ctx context.Context) *goredis.PubSub {
	return r.rdb.Subscribe(ctx, pushChannel(r.instanceID))
}

// CrossPushPayload wraps push data with routing hints.
type CrossPushPayload struct {
	UserID       int64  `json:"user_id"`
	ExceptDevice string `json:"except_device,omitempty"`
	OnlyDevice   string `json:"only_device,omitempty"`
	Data         json.RawMessage `json:"data"`
}
