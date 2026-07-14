package main

import (
	"context"
	"log"

	"squirtlechat/internal/app"
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
	r := gin.Default()
	c.RegisterHTTP(r)
	log.Printf("gateway-http :%s", c.Config.HTTPPort)
	if err := r.Run(":" + c.Config.HTTPPort); err != nil {
		log.Fatal(err)
	}
}
