package kafka

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
)

const errConsumerNotConnected = "consumer is not connected to Kafka"

// Consumer represents the consumer instance which handles Kafka messages
type Consumer interface {
	// StartListening accepts a function which will get called for each incoming message.
	StartListening(messageHandler func(message FTMessage) error)

	// Shutdown must be called before the application exits to stop the consumer instance.
	Shutdown()

	// ConnectivityCheck checks if the consumer can connect to the Kafka broker.
	ConnectivityCheck() error
}

// messageConsumer represents the library's main kafka consumer
type messageConsumer struct {
	topics                  []string
	consumerGroup           string
	brokersConnectionString string
	consumer                sarama.ConsumerGroup
	config                  *sarama.Config
	logger                  *logger.UPPLogger
	handler                 *ConsumerHandler
	closed                  chan struct{}
}

// Config keeps together all the values needed to create a consumer instance
type Config struct {
	BrokersConnectionString string
	ConsumerGroup           string
	Topics                  []string
	ConsumerGroupConfig     *sarama.Config
	Logger                  *logger.UPPLogger
}

// NewConsumer creates a new consumer instance using a Sarama ConsumerGroup
// to connect to Kafka.
func NewConsumer(config Config) (Consumer, error) {
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
		consumerGroup:           config.ConsumerGroup,
		brokersConnectionString: config.BrokersConnectionString,
		consumer:                consumer,
		config:                  config.ConsumerGroupConfig,
		logger:                  config.Logger,
		closed:                  make(chan struct{}),
	}, nil
}

// StartListening will start listening for message from Kafka.
func (c *messageConsumer) StartListening(messageHandler func(message FTMessage) error) {
	if c.handler == nil {
		c.handler = NewConsumerHandler(c.logger, messageHandler)
	}

	go func() {
		for err := range c.consumer.Errors() {
			c.logger.WithError(err).
				WithField("method", "StartListening").
				Error("error processing message")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			if err := c.consumer.Consume(ctx, c.topics, c.handler); err != nil {
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

// Shutdown closes the consumer's connection to Kafka
// It should be called before terminating the process.
func (c *messageConsumer) Shutdown() {
	close(c.closed)

	if err := c.consumer.Close(); err != nil {
		c.logger.WithError(err).
			WithField("method", "Shutdown").
			Error("Error closing consumer")
	}
}

// ConnectivityCheck tries to establish a new Kafka connection with a separate consumer group
// The consumer's existing connection is automatically repaired after any interruption.
func (c *messageConsumer) ConnectivityCheck() error {
	config := Config{
		BrokersConnectionString: c.brokersConnectionString,
		ConsumerGroup:           fmt.Sprintf("%s-healthcheck-%d", c.consumerGroup, rand.Intn(100)),
		Topics:                  c.topics,
		ConsumerGroupConfig:     c.config,
		Logger:                  c.logger,
	}
	healthcheckConsumer, err := NewConsumer(config)
	if err != nil {
		return err
	}
	defer healthcheckConsumer.Shutdown()

	return nil
}

// DefaultConsumerConfig returns a new sarama configuration with predefined default settings.
func DefaultConsumerConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	return config
}
