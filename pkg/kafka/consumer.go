package kafka

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
)

const errConsumerNotConnected = "consumer is not connected to Kafka"

// messageConsumer represents the library's main kafka consumer
type messageConsumer struct {
	topics                  []string
	consumerGroupName       string
	brokersConnectionString string
	consumerGroup           sarama.ConsumerGroup
	config                  *sarama.Config
	logger                  *logger.UPPLogger
	handler                 *ConsumerHandler
	closed                  chan struct{}
}

// consumerConfig keeps together all the values needed to create a consumer instance
type consumerConfig struct {
	BrokersConnectionString string
	ConsumerGroup           string
	Topics                  []string
	ConsumerGroupConfig     *sarama.Config
	Logger                  *logger.UPPLogger
}

// newConsumer creates a new consumer instance using a Sarama ConsumerGroup
// to connect to Kafka.
func newConsumer(config consumerConfig) (*messageConsumer, error) {
	config.Logger.Debug("Creating new consumer")

	if config.ConsumerGroupConfig == nil {
		config.ConsumerGroupConfig = DefaultConsumerConfig()
	}

	consumer, err := sarama.NewConsumerGroup(strings.Split(config.BrokersConnectionString, ","), config.ConsumerGroup, config.ConsumerGroupConfig)

	if err != nil {
		config.Logger.WithError(err).
			WithField("method", "NewConsumer").
			Error("Error creating Kafka consumer")
		return nil, err
	}

	return &messageConsumer{
		topics:                  config.Topics,
		consumerGroupName:       config.ConsumerGroup,
		brokersConnectionString: config.BrokersConnectionString,
		consumerGroup:           consumer,
		config:                  config.ConsumerGroupConfig,
		logger:                  config.Logger,
		closed:                  make(chan struct{}),
	}, nil
}

// startListening will start listening for message from Kafka.
func (c *messageConsumer) startListening(messageHandler func(message FTMessage)) {
	if c.handler == nil {
		c.handler = NewConsumerHandler(c.logger, messageHandler)
	}

	go func() {
		for err := range c.consumerGroup.Errors() {
			c.logger.WithError(err).
				WithField("method", "StartListening").
				Error("error processing message")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			if err := c.consumerGroup.Consume(ctx, c.topics, c.handler); err != nil {
				c.logger.WithError(err).
					WithField("method", "StartListening").
					Error("error starting consumer")
			}
			// check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-c.handler.ready:
				c.logger.Debug("New consumer group session starting...")
			case <-c.closed:
				// Terminate the message consumption.
				cancel()
				return
			}
		}
	}()

	c.logger.Info("Starting consumer...")
}

// close closes the consumer's connection to Kafka
// It should be called before terminating the process.
func (c *messageConsumer) close() error {
	close(c.closed)

	return c.consumerGroup.Close()
}

// connectivityCheck tries to establish a new Kafka connection with a separate consumer group
// The consumer's existing connection is automatically repaired after any interruption.
func (c *messageConsumer) connectivityCheck() error {
	config := consumerConfig{
		BrokersConnectionString: c.brokersConnectionString,
		ConsumerGroup:           fmt.Sprintf("healthcheck-%d", rand.Intn(100)),
		Topics:                  c.topics,
		ConsumerGroupConfig:     c.config,
		Logger:                  c.logger,
	}
	healthCheckConsumer, err := newConsumer(config)
	if err != nil {
		return err
	}
	_ = healthCheckConsumer.close()

	return nil
}

// DefaultConsumerConfig returns a new sarama configuration with predefined default settings.
func DefaultConsumerConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.MaxProcessingTime = 10 * time.Second
	return config
}
