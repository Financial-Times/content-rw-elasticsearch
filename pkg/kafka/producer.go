package kafka

import (
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
	"strings"
)

const errProducerNotConnected = "producer is not connected to Kafka"

// messageProducer implements the Producer interface and is the library's main producer implementation.
type messageProducer struct {
	brokers  []string
	topic    string
	config   *sarama.Config
	producer sarama.SyncProducer
	logger   *logger.UPPLogger
}

// newProducer creates a new producer instance using Sarama's SyncProducer.
func newProducer(brokers string, topic string, config *sarama.Config, logger *logger.UPPLogger) (*messageProducer, error) {
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

// sendMessage sends a message to Kafka.
func (p *messageProducer) sendMessage(message FTMessage) error {
	_, _, err := p.producer.SendMessage(&sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.StringEncoder(message.Build()),
	})

	return err
}

// close closes the producer's connection.
func (p *messageProducer) close() error {
	return p.producer.Close()
}

// connectivityCheck tries to create a new producer, then shuts it down if successful.
func (p *messageProducer) connectivityCheck() error {
	// like the consumer check, establishing a new connection gives us some degree of confidence
	healthCheckProducer, err := newProducer(strings.Join(p.brokers, ","), p.topic, p.config, p.logger)
	if err != nil {
		return err
	}

	_ = healthCheckProducer.close()

	return nil
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
