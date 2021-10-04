package kafka

import (
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
	"sync"
)

// ConsumerHandler represents a Sarama consumer group consumer.
type ConsumerHandler struct {
	sync.RWMutex
	ready   chan bool
	logger  *logger.UPPLogger
	handler func(message FTMessage) error
}

// NewConsumerHandler creates a new ConsumerHandler.
func NewConsumerHandler(logger *logger.UPPLogger, handler func(message FTMessage) error) *ConsumerHandler {
	return &ConsumerHandler{
		RWMutex: sync.RWMutex{},
		ready:   make(chan bool),
		logger:  logger,
		handler: handler,
	}
}

func (c *ConsumerHandler) Ready() <-chan bool {
	c.Lock()
	defer c.Unlock()

	return c.ready
}

func (c *ConsumerHandler) Reset() {
	c.Lock()
	defer c.Unlock()

	c.ready = make(chan bool)
}

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (c *ConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	close(c.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited.
func (c *ConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim claims a single topic partition and handles the messages that get added in it.
func (c *ConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	c.logger.
		WithField("method", "ConsumeClaim").
		Infof("claimed partition %d", claim.Partition())

	for message := range claim.Messages() {
		c.logger.
			WithField("method", "ConsumeClaim").
			Info("consuming message")

		ftMsg := rawToFTMessage(message.Value)
		err := c.handler(ftMsg)
		if err != nil {
			c.logger.WithError(err).
				WithField("method", "ConsumeClaim").
				WithField("messageKey", message.Key).
				Error("Error processing message")
			return err
		}
		session.MarkMessage(message, "")
	}

	c.logger.
		WithField("method", "ConsumeClaim").
		Infof("released claim on partition %d", claim.Partition())

	return nil
}
