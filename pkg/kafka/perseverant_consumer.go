package kafka

import (
	"fmt"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
	"sync"
	"time"
)

// perseverantConsumer implements the Consumer interface by creating
// a consumer which will keep trying to reconnect to Kafka on a specified interval.
// The underlying consumer is created lazily when perseverantConsumer starts listening for messages.
type perseverantConsumer struct {
	sync.RWMutex
	consumerGroup           string
	topics                  []string
	config                  *sarama.Config
	consumer                Consumer
	retryInterval           time.Duration
	errCh                   *chan error
	logger                  *logger.UPPLogger
	brokersConnectionString string
}

// NewPerseverantConsumer creates a perseverantConsumer
func NewPerseverantConsumer(config Config, retryInterval time.Duration) (Consumer, error) {
	consumer := &perseverantConsumer{
		RWMutex:                 sync.RWMutex{},
		consumerGroup:           config.ConsumerGroup,
		topics:                  config.Topics,
		config:                  config.ConsumerGroupConfig,
		errCh:                   &config.Err,
		logger:                  config.Logger,
		brokersConnectionString: config.BrokersConnectionString,
		retryInterval:           retryInterval,
	}
	return consumer, nil
}

// connect will attempt to create a new consumer continuously until successful.
func (c *perseverantConsumer) connect() {
	connectorLog := c.logger.WithField("brokers", c.brokersConnectionString).
		WithField("topics", c.topics).
		WithField("consumerGroup", c.consumerGroup)
	for {
		var errCh chan error
		if c.errCh != nil {
			errCh = *c.errCh
		}

		consumer, err := NewConsumer(Config{
			BrokersConnectionString: c.brokersConnectionString,
			ConsumerGroup:           c.consumerGroup,
			Topics:                  c.topics,
			ConsumerGroupConfig:     c.config,
			Err:                     errCh,
			Logger:                  c.logger,
		})

		if err == nil {
			connectorLog.Info("connected to Kafka consumer")
			c.setConsumer(consumer)
			break
		}

		connectorLog.WithError(err).Warn(errConsumerNotConnected)
		time.Sleep(c.retryInterval)
	}
}

// setConsumer sets the perseverantConsumer's consumer instance
func (c *perseverantConsumer) setConsumer(consumer Consumer) {
	c.Lock()
	defer c.Unlock()

	c.consumer = consumer
}

// isConnected returns if the consumer property is set.
// It is only set if a successful connection is established.
func (c *perseverantConsumer) isConnected() bool {
	c.RLock()
	defer c.RUnlock()

	return c.consumer != nil
}

// StartListening is a blocking call that tries to establish a connection to Kafka then starts listening.
func (c *perseverantConsumer) StartListening(messageHandler func(message FTMessage) error) {
	if !c.isConnected() {
		c.connect()
	}

	c.RLock()
	defer c.RUnlock()

	c.consumer.StartListening(messageHandler)
}

// Shutdown closes the consumer connection if it exists.
func (c *perseverantConsumer) Shutdown() {
	c.RLock()
	defer c.RUnlock()

	if c.isConnected() {
		c.consumer.Shutdown()
	}
}

// ConnectivityCheck tests if the consumer instance is created, then checks if it can connect to Kafka.
func (c *perseverantConsumer) ConnectivityCheck() error {
	c.RLock()
	defer c.RUnlock()

	if !c.isConnected() {
		return fmt.Errorf(errConsumerNotConnected)
	}

	return c.consumer.ConnectivityCheck()
}
