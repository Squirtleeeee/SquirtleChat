package app

import (
	"context"
	"encoding/json"
	"log"

	"squirtlechat/internal/handler"
	"squirtlechat/internal/push"
	"squirtlechat/internal/service"
	"squirtlechat/internal/store"
	"squirtlechat/internal/ws"
	"squirtlechat/pkg/config"
	"squirtlechat/pkg/database"
	"squirtlechat/pkg/idgen"
	pkgkafka "squirtlechat/pkg/kafka"
	pkgredis "squirtlechat/pkg/redis"
	"squirtlechat/pkg/routing"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
)

type Container struct {
	Config     *config.Config
	Auth       *service.AuthService
	Message    *service.MessageService
	Friend     *service.FriendService
	Sync       *service.SyncService
	File       *service.FileService
	Agent      *service.AgentService
	Hub        *ws.Hub
	Router     *routing.Router
	Dispatcher *push.Dispatcher
	Redis      *goredis.Client
}

func Bootstrap() (*Container, error) {
	cfg := config.Load()
	db, err := database.Open(cfg.MySQLDSN)
	if err != nil {
		return nil, err
	}
	rdb := pkgredis.New(cfg.RedisAddr)
	if err := pkgredis.Ping(context.Background(), rdb); err != nil {
		log.Printf("redis ping warn: %v", err)
	}

	gen := idgen.New(1)
	userStore := store.NewUserStore(db)
	msgStore := store.NewMessageStore(db)
	friendStore := store.NewFriendStore(db)
	syncStore := store.NewSyncStore(db)
	fileStore := store.NewFileStore(db)

	authSvc := service.NewAuthService(userStore, gen, cfg.JWTSecret)
	writer := pkgkafka.NewWriter(cfg.KafkaBroker, pkgkafka.TopicMessageSent)
	msgSvc := service.NewMessageService(msgStore, friendStore, gen, writer, cfg.GatewayInstanceID)
	friendSvc := service.NewFriendService(friendStore, userStore, gen)
	syncSvc := service.NewSyncService(syncStore, msgStore, userStore)
	fileSvc := service.NewFileService(fileStore, gen, cfg)
	agentSvc := service.NewAgentService(userStore, friendStore, msgStore, msgSvc, gen, cfg)
	if err := agentSvc.Init(context.Background()); err != nil {
		log.Printf("agent init warn: %v", err)
	}
	if cfg.LLMAPIKey != "" {
		log.Printf("llm configured base=%s model=%s", cfg.LLMAPIBase, cfg.LLMModel)
	} else {
		log.Printf("llm not configured (set deploy/llm.env or LLM_API_KEY)")
	}

	router := routing.New(rdb, cfg.GatewayInstanceID)
	hub := ws.NewHub(authSvc, &wsMsgAdapter{svc: msgSvc}, router)
	dispatcher := push.NewDispatcher(hub, router, msgStore)
	syncSvc.SetOnRead(func(ctx context.Context, readerID int64, convID string, readSeq int64) {
		dispatcher.BroadcastRead(ctx, readerID, convID, readSeq)
	})

	return &Container{
		Config:     cfg,
		Auth:       authSvc,
		Message:    msgSvc,
		Friend:     friendSvc,
		Sync:       syncSvc,
		File:       fileSvc,
		Agent:      agentSvc,
		Hub:        hub,
		Router:     router,
		Dispatcher: dispatcher,
		Redis:      rdb,
	}, nil
}

func (c *Container) RegisterHTTP(r *gin.Engine) {
	r.GET("/health", func(ctx *gin.Context) {
		status := gin.H{"status": "ok", "service": "gateway-http", "instance": c.Config.GatewayInstanceID}
		if err := pkgredis.Ping(ctx.Request.Context(), c.Redis); err != nil {
			status["redis"] = "down"
		} else {
			status["redis"] = "ok"
		}
		ctx.JSON(200, status)
	})
	api := r.Group("/api/v1")
	fileHandler := handler.NewFileHandler(c.File, c.Auth)
	handler.NewAuthHandler(c.Auth, c.File, c.Agent).Register(api)
	handler.NewFriendHandler(c.Friend, c.Auth).Register(api)
	handler.NewMessageHandler(c.Message, c.Auth).Register(api)
	handler.NewSyncHandler(c.Sync, c.Auth).Register(api)
	fileHandler.Register(api)
	fileHandler.RegisterPublic(r)
	handler.NewAgentHandler(c.Agent, c.Auth).Register(api)
}

type wsMsgAdapter struct {
	svc *service.MessageService
}

func (a *wsMsgAdapter) HandleSend(userID int64, deviceID string, raw json.RawMessage) ([]byte, error) {
	return a.svc.HandleSend(context.Background(), userID, deviceID, raw)
}

func (a *wsMsgAdapter) HandleTyping(userID int64, deviceID string, raw json.RawMessage) error {
	return a.svc.HandleTyping(context.Background(), userID, deviceID, raw)
}

func init() { log.Println("squirtlechat bootstrap") }
