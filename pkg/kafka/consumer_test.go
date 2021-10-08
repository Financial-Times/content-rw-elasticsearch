package kafka

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
)

const (
	brokerURL         = "localhost:29092"
	testConsumerGroup = "testgroup"
)

var messages = []*sarama.ConsumerMessage{{Value: []byte("Message1")}, {Value: []byte("Message2")}}

func TestNewConsumer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test as it requires a connection to Kafka.")
	}
	config := DefaultConsumerConfig()

	var buf bytes.Buffer
	log := logger.NewUPPLogger("test", "INFO")
	log.Out = &buf

	consumer, err := NewConsumer(Config{
		BrokersConnectionString: brokerURL,
		ConsumerGroup:           testConsumerGroup,
		Topics:                  []string{testTopic},
		ConsumerGroupConfig:     config,
		Logger:                  log,
	})
	assert.NoError(t, err)

	err = consumer.ConnectivityCheck()
	assert.NoError(t, err)

	consumer.Shutdown()
}

func TestConsumerNotConnectedConnectivityCheckError(t *testing.T) {
	log := logger.NewUPPLogger("test", "INFO")
	consumer := messageConsumer{brokersConnectionString: "unknown:9092", consumerGroup: testConsumerGroup, topics: []string{testTopic}, config: nil, logger: log}

	err := consumer.ConnectivityCheck()
	assert.Error(t, err)
}

func TestNewPerseverantConsumer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test as it requires a connection to Kafka.")
	}

	config := DefaultConsumerConfig()
	log := logger.NewUPPLogger("test", "INFO")

	consumer, err := NewPerseverantConsumer(Config{
		BrokersConnectionString: brokerURL,
		ConsumerGroup:           testConsumerGroup,
		Topics:                  []string{testTopic},
		ConsumerGroupConfig:     config,
		Logger:                  log,
	}, time.Second)
	assert.NoError(t, err)

	err = consumer.ConnectivityCheck()
	assert.EqualError(t, err, errConsumerNotConnected)

	consumer.StartListening(func(msg FTMessage) error { return nil })

	time.Sleep(3 * time.Second)

	err = consumer.ConnectivityCheck()
	assert.NoError(t, err)

	time.Sleep(time.Second)

	consumer.Shutdown()
}

type MockConsumerGroupClaim struct {
	messages []*sarama.ConsumerMessage
}

func (c *MockConsumerGroupClaim) Topic() string {
	return ""
}

func (c *MockConsumerGroupClaim) Partition() int32 {
	return 0
}

func (c *MockConsumerGroupClaim) InitialOffset() int64 {
	return 0
}

func (c *MockConsumerGroupClaim) HighWaterMarkOffset() int64 {
	return 0
}

func (c *MockConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage {
	outChan := make(chan *sarama.ConsumerMessage, len(c.messages))
	defer close(outChan)

	for _, v := range c.messages {
		outChan <- v
	}

	return outChan
}

type MockConsumerGroup struct {
	errChan         chan error
	messages        []*sarama.ConsumerMessage
	errors          []error
	IsShutdown      bool
	errorOnShutdown bool
}

func (cg *MockConsumerGroup) Errors() <-chan error {
	return cg.errChan
}

func (cg *MockConsumerGroup) Close() error {
	cg.IsShutdown = true
	if cg.errorOnShutdown {
		return fmt.Errorf("foobar")
	}
	return nil
}

func (cg *MockConsumerGroup) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	for _, v := range cg.messages {
		session := &MockConsumerGroupSession{}
		claim := &MockConsumerGroupClaim{
			messages: []*sarama.ConsumerMessage{v},
		}

		err := handler.ConsumeClaim(session, claim)
		if err != nil {
			cg.errChan <- err
		}
	}

	// We block here to simulate the behavior of the library
	c := make(chan struct{})
	<-c
	return nil
}

type MockConsumerGroupSession struct{}

func (m *MockConsumerGroupSession) Claims() map[string][]int32 {
	return make(map[string][]int32)
}

func (m *MockConsumerGroupSession) MemberID() string {
	return ""
}

func (m *MockConsumerGroupSession) GenerationID() int32 {
	return 1
}

func (m *MockConsumerGroupSession) MarkOffset(topic string, partition int32, offset int64, metadata string) {

}

func (m *MockConsumerGroupSession) Commit() {

}

func (m *MockConsumerGroupSession) ResetOffset(topic string, partition int32, offset int64, metadata string) {

}

func (m *MockConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {

}

func (m *MockConsumerGroupSession) Context() context.Context {
	return context.TODO()
}

func NewTestConsumer() Consumer {
	log := logger.NewUPPLogger("test", "INFO")
	return &messageConsumer{
		topics:                  []string{"topic"},
		consumerGroup:           "group",
		brokersConnectionString: "node",
		consumer: &MockConsumerGroup{
			messages:   messages,
			errors:     []error{},
			IsShutdown: false,
			errChan:    make(chan error),
		},
		logger: log,
		closed: make(chan struct{}),
	}
}

func TestErrorDuringShutdown(t *testing.T) {
	var buf bytes.Buffer
	l := logger.NewUPPLogger("test", "INFO")
	l.Out = &buf

	consumer := NewTestConsumerWithErrOnShutdown(l)

	consumer.Shutdown()

	assert.Equal(t, true, strings.Contains(buf.String(), "Error closing consumer"))
}

func NewTestConsumerWithErrOnShutdown(log *logger.UPPLogger) Consumer {
	return &messageConsumer{
		topics:                  []string{"topic"},
		consumerGroup:           "group",
		brokersConnectionString: brokerURL,
		consumer: &MockConsumerGroup{
			messages:        messages,
			errors:          []error{},
			IsShutdown:      false,
			errorOnShutdown: true,
		},
		logger: log,
		closed: make(chan struct{}),
	}
}

func TestMessageConsumer_StartListening(t *testing.T) {
	var count int32
	consumer := NewTestConsumer()

	consumer.StartListening(func(msg FTMessage) error {
		atomic.AddInt32(&count, 1)
		return nil
	})

	time.Sleep(1 * time.Second)
	assert.Equal(t, int32(len(messages)), atomic.LoadInt32(&count))
}

func TestMessageConsumerContinuesWhenHandlerReturnsError(t *testing.T) {
	var count int32
	consumer := NewTestConsumer()

	consumer.StartListening(func(msg FTMessage) error {
		atomic.AddInt32(&count, 1)
		return fmt.Errorf("test error")
	})

	time.Sleep(1 * time.Second)
	assert.Equal(t, int32(len(messages)), atomic.LoadInt32(&count))
}

func TestPerseverantConsumerListensToConsumer(t *testing.T) {
	var count int32
	consumer := perseverantConsumer{consumer: NewTestConsumer()}

	consumer.StartListening(func(msg FTMessage) error {
		atomic.AddInt32(&count, 1)
		return nil
	})

	time.Sleep(1 * time.Second)
	assert.Equal(t, int32(len(messages)), atomic.LoadInt32(&count))

	consumer.Shutdown()
}
