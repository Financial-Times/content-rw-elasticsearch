package kafka

import (
	"errors"
	"sync"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
)

// PerseverantProducer implements the Producer interface.
// It will attempt to create a new producer instance continuously until successful.
type PerseverantProducer struct {
	sync.RWMutex
	brokers  string
	topic    string
	config   *sarama.Config
	producer *messageProducer
	logger   *logger.UPPLogger
}

// NewPerseverantProducer creates a new PerseverantProducer
func NewPerseverantProducer(brokers string, topic string, config *sarama.Config, initialDelay time.Duration, retryInterval time.Duration, logger *logger.UPPLogger) (*PerseverantProducer, error) {
	producer := &PerseverantProducer{sync.RWMutex{}, brokers, topic, config, nil, logger}

	go func() {
		if initialDelay > 0 {
			time.Sleep(initialDelay)
		}
		producer.connect(retryInterval)
	}()

	return producer, nil
}

// connect tries to establish a connection to Kafka and will retry endlessly.
func (p *PerseverantProducer) connect(retryInterval time.Duration) {
	connectorLog := p.logger.WithField("brokers", p.brokers).
		WithField("topic", p.topic)
	for {
		producer, err := newProducer(p.brokers, p.topic, p.config, p.logger)
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

// setProducer sets the underlying producer instance.
func (p *PerseverantProducer) setProducer(producer *messageProducer) {
	p.Lock()
	defer p.Unlock()

	p.producer = producer
}

// isConnected checks if the underlying producer instance is set.
func (p *PerseverantProducer) isConnected() bool {
	p.RLock()
	defer p.RUnlock()

	return p.producer != nil
}

// SendMessage checks if the producer is connected, then sends a message to Kafka.
func (p *PerseverantProducer) SendMessage(message FTMessage) error {
	if !p.isConnected() {
		return errors.New(errProducerNotConnected)
	}

	p.RLock()
	defer p.RUnlock()

	return p.producer.sendMessage(message)
}

// Close closes the connection to Kafka if the producer is connected
func (p *PerseverantProducer) Close() error {
	if p.isConnected() {
		return p.producer.close()
	}

	return nil
}

// ConnectivityCheck checks if the producer has established connection to Kafka.
func (p *PerseverantProducer) ConnectivityCheck() error {
	if !p.isConnected() {
		return errors.New(errProducerNotConnected)
	}

	return p.producer.connectivityCheck()
}
