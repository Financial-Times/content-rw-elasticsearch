package kafka

import (
	logger "github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
	"strings"
)

const errProducerNotConnected = "producer is not connected to Kafka"

// Producer represents the producer instance which sends Kafka messages.
type Producer interface {
	SendMessage(message FTMessage) error
	ConnectivityCheck() error
	Shutdown()
}

// messageProducer implements the Producer interface and is the library's main producer implementation.
type messageProducer struct {
	brokers  []string
	topic    string
	config   *sarama.Config
	producer sarama.SyncProducer
	logger   *logger.UPPLogger
}

// NewProducer creates a new producer instance using Sarama's SyncProducer.
func NewProducer(brokers string, topic string, config *sarama.Config, logger *logger.UPPLogger) (Producer, error) {
	if config == nil {
		config = DefaultProducerConfig()
	}

	brokerSlice := strings.Split(brokers, ",")

	sp, err := sarama.NewSyncProducer(brokerSlice, config)
	if err != nil {
		logger.WithError(err).
			WithField("method", "NewProducer").
			Error("Error creating the producer")
		return nil, err
	}

	return &messageProducer{
		brokers:  brokerSlice,
		topic:    topic,
		config:   config,
		producer: sp,
		logger:   logger,
	}, nil
}

// SendMessage sends a message to Kafka.
func (p *messageProducer) SendMessage(message FTMessage) error {
	_, _, err := p.producer.SendMessage(&sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.StringEncoder(message.Build()),
	})
	if err != nil {
		p.logger.WithError(err).
			WithField("method", "SendMessage").
			Error("Error sending a Kafka message")
	}
	return err
}

// Shutdown closes the producer's connection.
func (p *messageProducer) Shutdown() {
	if err := p.producer.Close(); err != nil {
		p.logger.WithError(err).
			WithField("method", "Shutdown").
			Error("Error closing the producer")
	}
}

// ConnectivityCheck tries to create a new producer, then shuts it down if successful.
func (p *messageProducer) ConnectivityCheck() error {
	// like the consumer check, establishing a new connection gives us some degree of confidence
	tmp, err := NewProducer(strings.Join(p.brokers, ","), p.topic, p.config, p.logger)
	if tmp != nil {
		defer tmp.Shutdown()
	}

	return err
}

// DefaultProducerConfig creates a new Sarama producer configuration with default values.
func DefaultProducerConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Producer.MaxMessageBytes = 16777216
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 10
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	return config
}
