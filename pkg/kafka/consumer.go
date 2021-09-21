package kafka

import (
	"context"
	"strings"
	"sync"

	logger "github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
)

const errConsumerNotConnected = "consumer is not connected to Kafka"

type ConsumerGroupClaimer interface {
	SetHandler(handler func(message FTMessage) error)
	Ready() <-chan bool
	Reset()
	Setup(session sarama.ConsumerGroupSession) error
	Cleanup(session sarama.ConsumerGroupSession) error
	ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error
}

type Consumer interface {
	StartListening(messageHandler func(message FTMessage) error)
	Shutdown()
	ConnectivityCheck() error
}

type MessageConsumer struct {
	topics        []string
	consumerGroup string
	brokers       []string
	consumer      sarama.ConsumerGroup
	config        *sarama.Config
	errCh         chan error
	logger        *logger.UPPLogger
	claimer       ConsumerGroupClaimer
}

type Config struct {
	BrokersConnectionString string
	ConsumerGroup           string
	Topics                  []string
	ConsumerGroupConfig     *sarama.Config
	Err                     chan error
	Logger                  *logger.UPPLogger
}

func NewConsumer(config Config) (Consumer, error) {
	config.Logger.Debug("Creating new consumer")

	if config.ConsumerGroupConfig == nil {
		config.ConsumerGroupConfig = DefaultConsumerConfig()
	}

	consumer, err := sarama.NewConsumerGroup(strings.Split(config.BrokersConnectionString, ","), config.ConsumerGroup, config.ConsumerGroupConfig)

	if err != nil {
		config.Logger.WithError(err).
			WithField("method", "NewConsumer").
			Error("Error creating Kafka consumer")
		return nil, err
	}

	return &MessageConsumer{
		topics:        config.Topics,
		consumerGroup: config.ConsumerGroup,
		brokers:       strings.Split(config.BrokersConnectionString, ","),
		consumer:      consumer,
		config:        config.ConsumerGroupConfig,
		errCh:         config.Err,
		logger:        config.Logger,
	}, nil
}

func (c *MessageConsumer) StartListening(messageHandler func(message FTMessage) error) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if c.claimer == nil {
		c.claimer = NewConsumerClient(c.logger)
	}

	c.claimer.SetHandler(messageHandler)

	go func() {
		c.logger.Debug("Start listening for consumer errors")
		for err := range c.consumer.Errors() {
			c.logger.WithError(err).
				WithField("method", "StartListening").
				Error("error processing message")

			if c.errCh != nil {
				c.errCh <- err
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			if err := c.consumer.Consume(ctx, c.topics, c.claimer); err != nil {
				c.logger.WithError(err).
					WithField("method", "StartListening").
					Error("error starting consumer")
			}
			// check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				return
			}
			c.claimer.Reset()
		}
	}()

	<-c.claimer.Ready()
	c.logger.Info("consumer up and running")
	wg.Wait()
}

func (c *MessageConsumer) Shutdown() {
	if err := c.consumer.Close(); err != nil {
		c.logger.WithError(err).
			WithField("method", "Shutdown").
			Error("Error closing consumer")

		if c.errCh != nil {
			c.errCh <- err
		}
	}
}

func (c *MessageConsumer) ConnectivityCheck() error {
	// establishing (or failing to establish) a new connection (with a distinct consumer group) is a reasonable check
	// as experiment shows the consumer's existing connection is automatically repaired after any interruption
	config := Config{
		BrokersConnectionString: strings.Join(c.brokers, ","),
		ConsumerGroup:           c.consumerGroup + "-healthcheck",
		Topics:                  c.topics,
		ConsumerGroupConfig:     c.config,
		Logger:                  c.logger,
	}
	healthcheckConsumer, err := NewConsumer(config)
	if err != nil {
		return err
	}
	defer healthcheckConsumer.Shutdown()

	return nil
}

func DefaultConsumerConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	return config
}
