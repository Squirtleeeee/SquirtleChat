package main

import (
	"context"
	"encoding/json"
	"log"
	"os/signal"
	"syscall"

	"squirtlechat/internal/app"
	"squirtlechat/internal/service"
	"squirtlechat/internal/store"
	pkgkafka "squirtlechat/pkg/kafka"

	"github.com/gin-gonic/gin"
	"github.com/segmentio/kafka-go"
)

func main() {
	c, err := app.Bootstrap()
	if err != nil {
		log.Fatal(err)
	}
	r := gin.Default()
	r.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"status":   "ok",
			"service":  "gateway-ws",
			"instance": c.Config.GatewayInstanceID,
		})
	})
	c.Hub.RegisterRoutes(r)
	c.Message.SetOnSent(func(ctx context.Context, evt *store.SentEvent) {
		c.Dispatcher.HandleEvent(ctx, evt)
		if c.Agent != nil {
			c.Agent.OnUserMessage(ctx, evt)
		}
	})
	c.Message.SetOnTyping(func(ctx context.Context, evt *service.TypingEvent) {
		c.Dispatcher.BroadcastTyping(ctx, evt.ConversationID, evt.FromUserID, evt.Typing, evt.ToUserIDs)
	})
	c.Message.SetOnReaction(func(ctx context.Context, evt *service.ReactionEvent) {
		c.Dispatcher.BroadcastReaction(ctx, evt)
	})
	c.Message.SetOnPin(func(ctx context.Context, evt *service.PinEvent) {
		c.Dispatcher.BroadcastPin(ctx, evt)
	})
	c.Message.SetOnPollVote(func(ctx context.Context, evt *service.PollVoteEvent) {
		c.Dispatcher.BroadcastPollVote(ctx, evt)
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go consumeRedisPush(ctx, c)
	go consumeKafka(ctx, c)

	log.Printf("gateway-ws instance=%s :%s", c.Config.GatewayInstanceID, c.Config.WSPort)
	if err := r.Run(":" + c.Config.WSPort); err != nil {
		log.Fatal(err)
	}
}

func consumeRedisPush(ctx context.Context, c *app.Container) {
	sub := c.Router.Subscribe(ctx)
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			c.Dispatcher.HandleCrossPush(ctx, []byte(msg.Payload))
		}
	}
}

func consumeKafka(ctx context.Context, c *app.Container) {
	group := "squirtlechat-push-" + c.Config.GatewayInstanceID
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{c.Config.KafkaBroker},
		Topic:   pkgkafka.TopicMessageSent,
		GroupID: group,
	})
	defer reader.Close()
	log.Printf("kafka consumer group=%s", group)

	for {
		m, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("kafka read: %v", err)
			continue
		}
		var evt store.SentEvent
		if err := json.Unmarshal(m.Value, &evt); err != nil {
			continue
		}
		if evt.OriginInstance != "" && evt.OriginInstance == c.Config.GatewayInstanceID {
			continue
		}
		c.Dispatcher.HandleEvent(ctx, &evt)
	}
}
