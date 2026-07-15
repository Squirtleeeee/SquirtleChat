package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"

	"squirtlechat/internal/agent"
	"squirtlechat/internal/app"
	"squirtlechat/pkg/idgen"
)

func main() {
	c, err := app.Bootstrap()
	if err != nil {
		log.Fatal(err)
	}
	cfg := c.Config
	fmt.Println("=== LLM config ===")
	fmt.Printf("base=%s model=%s key=%v\n", cfg.LLMAPIBase, cfg.LLMModel, cfg.LLMAPIKey != "")

	cli := agent.NewClient(cfg.LLMAPIBase, cfg.LLMAPIKey, cfg.LLMModel)
	out, err := cli.Chat(context.Background(), []agent.ChatMessage{{Role: "user", Content: "你好"}})
	fmt.Println("direct:", out)
	fmt.Println("err:", err)

	ctx := context.Background()
	botID, _ := c.Agent.BotUserID(ctx)

	res, err := c.Auth.Login(ctx, "test_a", "test1234", "agentdiag", "agentdiag")
	if err != nil {
		log.Fatal("login:", err)
	}
	fmt.Println("user:", res.User.ID)
	token := res.Tokens.AccessToken
	convID := idgen.DirectConversationID(res.User.ID, botID)
	fmt.Println("conv:", convID)

	wsURL := fmt.Sprintf("ws://localhost:8081/ws?token=%s&device_id=agentdiag", token)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatal("ws:", err)
	}
	defer conn.Close()

	payload, _ := json.Marshal(map[string]interface{}{
		"client_msg_id":     fmt.Sprintf("diag-%d", time.Now().UnixNano()),
		"conversation_id":   convID,
		"conversation_type": 1,
		"to_user_id":        fmt.Sprintf("%d", botID),
		"msg_type":          1,
		"content":           "你好，智能体测试",
	})
	frame, _ := json.Marshal(map[string]interface{}{"type": "message", "payload": json.RawMessage(payload)})
	if err := conn.WriteMessage(websocket.TextMessage, frame); err != nil {
		log.Fatal("ws write:", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("ws read end:", err)
			break
		}
		fmt.Println("ws:", string(msg))
	}
}
