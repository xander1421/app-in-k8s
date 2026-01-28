package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/alexprut/twitter-clone/pkg/models"
)

const (
	ExchangeName = "twitter"

	// Queue names
	QueueFanoutHigh   = "twitter.fanout.high"
	QueueFanoutNormal = "twitter.fanout.normal"
	QueueSearchIndex  = "twitter.search.index"
	QueueNotifyPush   = "twitter.notify.push"
	QueueMediaProcess = "twitter.media.process"
)

type JobHandler func(job models.FanoutJob) error

type RabbitMQ struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	instanceID string

	handlers   map[string]JobHandler
	handlersMu sync.RWMutex

	url string
}

func NewRabbitMQ(url, instanceID string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("channel: %w", err)
	}

	rmq := &RabbitMQ{
		conn:       conn,
		channel:    ch,
		instanceID: instanceID,
		handlers:   make(map[string]JobHandler),
		url:        url,
	}

	if err := rmq.setup(); err != nil {
		rmq.Close()
		return nil, fmt.Errorf("setup: %w", err)
	}

	return rmq, nil
}

func (rmq *RabbitMQ) setup() error {
	// Declare exchange
	if err := rmq.channel.ExchangeDeclare(
		ExchangeName,
		"direct",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}

	// Declare queues
	queues := []string{QueueFanoutHigh, QueueFanoutNormal, QueueSearchIndex, QueueNotifyPush, QueueMediaProcess}
	for _, q := range queues {
		args := amqp.Table{
			"x-message-ttl": int32(3600000), // 1 hour TTL
		}

		// High priority queue gets higher priority
		if q == QueueFanoutHigh {
			args["x-max-priority"] = int32(10)
		}

		_, err := rmq.channel.QueueDeclare(
			q,
			true,  // durable
			false, // auto-delete
			false, // exclusive
			false, // no-wait
			args,
		)
		if err != nil {
			return fmt.Errorf("declare queue %s: %w", q, err)
		}

		// Bind queue to exchange
		if err := rmq.channel.QueueBind(q, q, ExchangeName, false, nil); err != nil {
			return fmt.Errorf("bind queue %s: %w", q, err)
		}
	}

	// Set QoS for fair dispatch across consumers
	if err := rmq.channel.Qos(
		10,    // prefetch count
		0,     // prefetch size
		false, // global
	); err != nil {
		return fmt.Errorf("qos: %w", err)
	}

	return nil
}

func (rmq *RabbitMQ) Close() error {
	if rmq.channel != nil {
		rmq.channel.Close()
	}
	if rmq.conn != nil {
		return rmq.conn.Close()
	}
	return nil
}

func (rmq *RabbitMQ) Health(ctx context.Context) error {
	if rmq.conn == nil || rmq.conn.IsClosed() {
		return fmt.Errorf("connection closed")
	}
	return nil
}

// Publish sends a job to a queue
func (rmq *RabbitMQ) Publish(ctx context.Context, queueName string, job models.FanoutJob) error {
	job.CreatedAt = time.Now()

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	priority := uint8(5) // default
	if job.Priority == "high" {
		priority = 10
	} else if job.Priority == "low" {
		priority = 1
	}

	return rmq.channel.PublishWithContext(
		ctx,
		ExchangeName,
		queueName,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         data,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			AppId:        rmq.instanceID,
			Priority:     priority,
		},
	)
}

// PublishFanout queues a tweet fanout job
func (rmq *RabbitMQ) PublishFanout(ctx context.Context, tweetID, authorID string, followerCount int) error {
	queue := QueueFanoutNormal
	priority := "normal"

	// Use high priority queue for viral potential
	if followerCount > 10000 {
		queue = QueueFanoutHigh
		priority = "high"
	}

	return rmq.Publish(ctx, queue, models.FanoutJob{
		ID:       fmt.Sprintf("fanout-%s-%d", tweetID, time.Now().UnixNano()),
		Type:     "fanout",
		TweetID:  tweetID,
		AuthorID: authorID,
		Priority: priority,
		Payload: map[string]interface{}{
			"follower_count": followerCount,
		},
	})
}

// PublishSearchIndex queues a search indexing job
func (rmq *RabbitMQ) PublishSearchIndex(ctx context.Context, tweetID string, content string) error {
	return rmq.Publish(ctx, QueueSearchIndex, models.FanoutJob{
		ID:       fmt.Sprintf("index-%s-%d", tweetID, time.Now().UnixNano()),
		Type:     "index",
		TweetID:  tweetID,
		Priority: "normal",
		Payload: map[string]interface{}{
			"content": content,
		},
	})
}

// PublishNotification queues a notification job
func (rmq *RabbitMQ) PublishNotification(ctx context.Context, userID, notifType, actorID, tweetID string) error {
	return rmq.Publish(ctx, QueueNotifyPush, models.FanoutJob{
		ID:       fmt.Sprintf("notify-%d", time.Now().UnixNano()),
		Type:     "notify",
		TweetID:  tweetID,
		Priority: "normal",
		Payload: map[string]interface{}{
			"user_id":    userID,
			"notif_type": notifType,
			"actor_id":   actorID,
		},
	})
}

// PublishMediaProcess queues a media processing job
func (rmq *RabbitMQ) PublishMediaProcess(ctx context.Context, mediaID, uploaderID, contentType string) error {
	return rmq.Publish(ctx, QueueMediaProcess, models.FanoutJob{
		ID:       fmt.Sprintf("media-%s-%d", mediaID, time.Now().UnixNano()),
		Type:     "media_process",
		Priority: "normal",
		Payload: map[string]interface{}{
			"media_id":     mediaID,
			"uploader_id":  uploaderID,
			"content_type": contentType,
		},
	})
}

// RegisterHandler registers a handler for a specific queue
func (rmq *RabbitMQ) RegisterHandler(queueName string, handler JobHandler) {
	rmq.handlersMu.Lock()
	rmq.handlers[queueName] = handler
	rmq.handlersMu.Unlock()
}

// StartConsumer starts consuming from a queue
func (rmq *RabbitMQ) StartConsumer(ctx context.Context, queueName string) error {
	rmq.handlersMu.RLock()
	handler, ok := rmq.handlers[queueName]
	rmq.handlersMu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler registered for queue %s", queueName)
	}

	msgs, err := rmq.channel.Consume(
		queueName,
		rmq.instanceID+"-"+queueName,
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					log.Printf("[%s] Channel closed, stopping consumer", queueName)
					return
				}

				var job models.FanoutJob
				if err := json.Unmarshal(msg.Body, &job); err != nil {
					log.Printf("[%s] Failed to unmarshal job: %v", queueName, err)
					msg.Nack(false, false)
					continue
				}

				if err := handler(job); err != nil {
					log.Printf("[%s] Handler error for job %s: %v", queueName, job.ID, err)
					msg.Nack(false, true)
				} else {
					msg.Ack(false)
				}
			}
		}
	}()

	log.Printf("[%s] Started consumer on instance %s", queueName, rmq.instanceID)
	return nil
}

// StartAllConsumers starts consumers for all registered handlers
func (rmq *RabbitMQ) StartAllConsumers(ctx context.Context) error {
	rmq.handlersMu.RLock()
	defer rmq.handlersMu.RUnlock()

	for queueName := range rmq.handlers {
		if err := rmq.StartConsumer(ctx, queueName); err != nil {
			return err
		}
	}
	return nil
}

// GetQueueStats returns queue statistics
func (rmq *RabbitMQ) GetQueueStats(queueName string) (messages, consumers int, err error) {
	q, err := rmq.channel.QueueInspect(queueName)
	if err != nil {
		return 0, 0, err
	}
	return q.Messages, q.Consumers, nil
}
