package main

import (
	"context"
	"log"

	"squirtlechat/internal/app"
	"squirtlechat/internal/service"
	"squirtlechat/internal/store"

	"github.com/gin-gonic/gin"
)

func main() {
	c, err := app.Bootstrap()
	if err != nil {
		log.Fatal(err)
	}
	c.Message.SetOnRecalled(func(ctx context.Context, evt *store.RecallEvent) {
		c.Dispatcher.BroadcastRecall(ctx, evt)
	})
	c.Message.SetOnEdited(func(ctx context.Context, evt *service.EditEvent) {
		c.Dispatcher.BroadcastEdit(ctx, evt)
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
	c.Message.SetOnReminder(func(ctx context.Context, evt *service.ReminderEvent) {
		c.Dispatcher.BroadcastReminder(ctx, evt)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Message.RunScheduleWorker(ctx)
	r := gin.Default()
	c.RegisterHTTP(r)
	log.Printf("gateway-http :%s", c.Config.HTTPPort)
	if err := r.Run(":" + c.Config.HTTPPort); err != nil {
		log.Fatal(err)
	}
}
