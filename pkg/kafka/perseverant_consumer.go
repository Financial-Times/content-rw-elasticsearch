package kafka

import (
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
	"sync"
	"time"
)

// PerseverantConsumer implements the Consumer interface by creating
// a consumer which will keep trying to reconnect to Kafka on a specified interval.
// The underlying consumer is created lazily when perseverantConsumer starts listening for messages.
type PerseverantConsumer struct {
	sync.RWMutex
	consumerGroup           string
	topics                  []string
	config                  *sarama.Config
	consumer                *messageConsumer
	retryInterval           time.Duration
	errCh                   *chan error
	logger                  *logger.UPPLogger
	brokersConnectionString string
}

type PerseverantConsumerConfig struct {
	BrokersConnectionString string
	ConsumerGroup           string
	Topics                  []string
	ConsumerGroupConfig     *sarama.Config
	Logger                  *logger.UPPLogger
	RetryInterval           time.Duration
}

// NewPerseverantConsumer creates a perseverantConsumer
func NewPerseverantConsumer(config PerseverantConsumerConfig) (*PerseverantConsumer, error) {
	consumer := &PerseverantConsumer{
		RWMutex:                 sync.RWMutex{},
		consumerGroup:           config.ConsumerGroup,
		topics:                  config.Topics,
		config:                  config.ConsumerGroupConfig,
		logger:                  config.Logger,
		brokersConnectionString: config.BrokersConnectionString,
		retryInterval:           config.RetryInterval,
	}
	return consumer, nil
}

// connect will attempt to create a new consumer continuously until successful.
func (c *PerseverantConsumer) connect() {
	connectorLog := c.logger.WithField("brokers", c.brokersConnectionString).
		WithField("topics", c.topics).
		WithField("consumerGroup", c.consumerGroup)
	for {
		consumer, err := newConsumer(consumerConfig{
			BrokersConnectionString: c.brokersConnectionString,
			ConsumerGroup:           c.consumerGroup,
			Topics:                  c.topics,
			ConsumerGroupConfig:     c.config,
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
func (c *PerseverantConsumer) setConsumer(consumer *messageConsumer) {
	c.Lock()
	defer c.Unlock()

	c.consumer = consumer
}

// isConnected returns if the consumer property is set.
// It is only set if a successful connection is established.
func (c *PerseverantConsumer) isConnected() bool {
	c.RLock()
	defer c.RUnlock()

	return c.consumer != nil
}

// StartListening is a blocking call that tries to establish a connection to Kafka then starts listening.
func (c *PerseverantConsumer) StartListening(messageHandler func(message FTMessage)) {
	if !c.isConnected() {
		c.connect()
	}

	c.RLock()
	defer c.RUnlock()

	c.consumer.startListening(messageHandler)
}

// Close closes the consumer connection if it exists.
func (c *PerseverantConsumer) Close() error {
	if c.isConnected() {
		c.RLock()
		defer c.RUnlock()
		return c.consumer.close()
	}

	return nil
}

// ConnectivityCheck tests if the consumer instance is created, then checks if it can connect to Kafka.
func (c *PerseverantConsumer) ConnectivityCheck() error {
	if !c.isConnected() {
		c.connect()
	}

	c.RLock()
	defer c.RUnlock()

	return c.consumer.connectivityCheck()
}
