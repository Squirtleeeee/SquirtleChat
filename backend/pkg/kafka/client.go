package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
)

const TopicMessageSent = "im.message.sent"

func NewWriter(broker, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:     kafka.TCP(broker),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
}

func NewReader(broker, topic, group string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker},
		Topic:   topic,
		GroupID: group,
	})
}

func Publish(ctx context.Context, w *kafka.Writer, key, value []byte) error {
	return w.WriteMessages(ctx, kafka.Message{Key: key, Value: value})
}
