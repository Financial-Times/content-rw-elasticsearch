package kafka

import (
	"fmt"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
	"sync"
	"time"
)

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

func NewPerseverantConsumer(brokersConnectionString string, consumerGroup string, topics []string, config *sarama.Config, retryInterval time.Duration, errCh *chan error, logger *logger.UPPLogger) (Consumer, error) {
	consumer := &perseverantConsumer{
		RWMutex:                 sync.RWMutex{},
		consumerGroup:           consumerGroup,
		topics:                  topics,
		config:                  config,
		retryInterval:           retryInterval,
		errCh:                   errCh,
		logger:                  logger,
		brokersConnectionString: brokersConnectionString,
	}
	return consumer, nil
}

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

func (c *perseverantConsumer) setConsumer(consumer Consumer) {
	c.Lock()
	defer c.Unlock()

	c.consumer = consumer
}

func (c *perseverantConsumer) isConnected() bool {
	c.RLock()
	defer c.RUnlock()

	return c.consumer != nil
}

func (c *perseverantConsumer) StartListening(messageHandler func(message FTMessage) error) {
	if !c.isConnected() {
		c.connect()
	}

	c.RLock()
	defer c.RUnlock()

	c.consumer.StartListening(messageHandler)
}

func (c *perseverantConsumer) Shutdown() {
	c.RLock()
	defer c.RUnlock()

	if c.isConnected() {
		c.consumer.Shutdown()
	}
}

func (c *perseverantConsumer) ConnectivityCheck() error {
	c.RLock()
	defer c.RUnlock()

	if !c.isConnected() {
		return fmt.Errorf(errConsumerNotConnected)
	}

	return c.consumer.ConnectivityCheck()
}
