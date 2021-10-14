package kafka

import (
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
)

// ConsumerHandler represents a Sarama consumer group consumer.
type ConsumerHandler struct {
	ready   chan struct{}
	logger  *logger.UPPLogger
	handler func(message FTMessage)
}

// NewConsumerHandler creates a new ConsumerHandler.
func NewConsumerHandler(logger *logger.UPPLogger, handler func(message FTMessage)) *ConsumerHandler {
	return &ConsumerHandler{
		ready:   make(chan struct{}),
		logger:  logger,
		handler: handler,
	}
}

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (c *ConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	c.ready <- struct{}{}
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
		ftMsg := rawToFTMessage(message.Value)
		c.handler(ftMsg)
		session.MarkMessage(message, "")
	}

	c.logger.
		WithField("method", "ConsumeClaim").
		Infof("released claim on partition %d", claim.Partition())

	return nil
}
