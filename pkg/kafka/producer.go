package kafka

import (
	"strings"
	"sync"
	"time"

	logger "github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
)

const errProducerNotConnected = "producer is not connected to Kafka"

type Producer interface {
	SendMessage(message FTMessage) error
	ConnectivityCheck() error
	Shutdown()
}

type MessageProducer struct {
	brokers  []string
	topic    string
	config   *sarama.Config
	producer sarama.SyncProducer
	logger   *logger.UPPLogger
}

type perseverantProducer struct {
	sync.RWMutex
	brokers  string
	topic    string
	config   *sarama.Config
	producer Producer
	logger   *logger.UPPLogger
}

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

	return &MessageProducer{
		brokers:  brokerSlice,
		topic:    topic,
		config:   config,
		producer: sp,
		logger:   logger,
	}, nil
}

func NewPerseverantProducer(brokers string, topic string, config *sarama.Config, initialDelay time.Duration, retryInterval time.Duration, logger *logger.UPPLogger) (Producer, error) {
	producer := &perseverantProducer{sync.RWMutex{}, brokers, topic, config, nil, logger}

	go func() {
		if initialDelay > 0 {
			time.Sleep(initialDelay)
		}
		producer.connect(retryInterval)
	}()

	return producer, nil
}

func (p *MessageProducer) SendMessage(message FTMessage) error {
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

func (p *MessageProducer) Shutdown() {
	if err := p.producer.Close(); err != nil {
		p.logger.WithError(err).
			WithField("method", "Shutdown").
			Error("Error closing the producer")
	}
}

func (p *MessageProducer) ConnectivityCheck() error {
	// like the consumer check, establishing a new connection gives us some degree of confidence
	tmp, err := NewProducer(strings.Join(p.brokers, ","), p.topic, p.config, p.logger)
	if tmp != nil {
		defer tmp.Shutdown()
	}

	return err
}

func (p *perseverantProducer) connect(retryInterval time.Duration) {
	connectorLog := p.logger.WithField("brokers", p.brokers).
		WithField("topic", p.topic)
	for {
		producer, err := NewProducer(p.brokers, p.topic, p.config, p.logger)
		if err == nil {
			connectorLog.Info("connected to Kafka producer")
			p.setProducer(producer)
			break
		}

		connectorLog.WithError(err).
			Warn(errProducerNotConnected)
		time.Sleep(retryInterval)
	}
}

func (p *perseverantProducer) setProducer(producer Producer) {
	p.Lock()
	defer p.Unlock()

	p.producer = producer
}

func (p *perseverantProducer) isConnected() bool {
	p.RLock()
	defer p.RUnlock()

	return p.producer != nil
}

func (p *perseverantProducer) SendMessage(message FTMessage) error {
	if !p.isConnected() {
		return errors.New(errProducerNotConnected)
	}

	p.RLock()
	defer p.RUnlock()

	return p.producer.SendMessage(message)
}

func (p *perseverantProducer) Shutdown() {
	if p.isConnected() {
		p.producer.Shutdown()
	}
}

func (p *perseverantProducer) ConnectivityCheck() error {
	if !p.isConnected() {
		return errors.New(errProducerNotConnected)
	}

	return p.producer.ConnectivityCheck()
}

func DefaultProducerConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Producer.MaxMessageBytes = 16777216
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 10
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	return config
}
