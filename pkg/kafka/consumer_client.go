package kafka

import (
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
)

// ConsumerClient represents a Sarama consumer group consumer
type ConsumerClient struct {
	ready   chan bool
	logger  *logger.UPPLogger
	handler func(message FTMessage) error
}

// NewConsumerClient creates a new ConsumerClient
func NewConsumerClient(logger *logger.UPPLogger) *ConsumerClient {
	return &ConsumerClient{
		ready:  make(chan bool),
		logger: logger,
	}
}

func (c *ConsumerClient) SetHandler(handler func(message FTMessage) error) {
	c.handler = handler
}

func (c *ConsumerClient) Ready() <-chan bool {
	return c.ready
}

func (c *ConsumerClient) Reset() {
	c.ready = make(chan bool)
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (c *ConsumerClient) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(c.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (c *ConsumerClient) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (c *ConsumerClient) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
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
		}
		session.MarkMessage(message, "")
	}

	c.logger.
		WithField("method", "ConsumeClaim").
		Infof("released claim on partition %d", claim.Partition())

	return nil
}
