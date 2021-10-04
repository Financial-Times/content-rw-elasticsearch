package kafka

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	logger "github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

const (
	brokerURL         = "localhost:29092"
	testConsumerGroup = "testgroup"
)

var expectedErrors = []error{errors.New("booster separation failure"), errors.New("payload missing")}
var messages = []*sarama.ConsumerMessage{{Value: []byte("Message1")}, {Value: []byte("Message2")}}

func TestNewConsumer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test as it requires a connection to Kafka.")
	}
	config := DefaultConsumerConfig()

	errCh := make(chan error, 1)
	defer close(errCh)

	log := logger.NewUPPLogger("test", "INFO")
	consumer, err := NewConsumer(Config{
		BrokersConnectionString: brokerURL,
		ConsumerGroup:           testConsumerGroup,
		Topics:                  []string{testTopic},
		ConsumerGroupConfig:     config,
		Err:                     errCh,
		Logger:                  log,
	})
	assert.NoError(t, err)

	err = consumer.ConnectivityCheck()
	assert.NoError(t, err)

	select {
	case actualError := <-errCh:
		assert.NotNil(t, actualError, "Was not expecting error from consumer.")
	default:
	}

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
	errCh := make(chan error, 1)
	defer close(errCh)
	log := logger.NewUPPLogger("test", "PANIC")

	consumer, err := NewPerseverantConsumer(Config{
		BrokersConnectionString: brokerURL,
		ConsumerGroup:           testConsumerGroup,
		Topics:                  []string{testTopic},
		ConsumerGroupConfig:     config,
		Err:                     errCh,
		Logger:                  log,
	}, time.Second)
	assert.NoError(t, err)

	err = consumer.ConnectivityCheck()
	assert.EqualError(t, err, errConsumerNotConnected)

	go func() {
		consumer.StartListening(func(msg FTMessage) error { return nil })
	}()

	time.Sleep(time.Second)

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
		return errors.New("foobar")
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

type MockConsumerGroupSession struct {}

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


func NewTestConsumerWithErrChan() (Consumer, chan error) {
	errCh := make(chan error, len(expectedErrors))
	log := logger.NewUPPLogger("test", "INFO")

	return &messageConsumer{
		topics:                  []string{"topic"},
		consumerGroup:           "group",
		brokersConnectionString: "node",
		consumer: &MockConsumerGroup{
			messages:        messages,
			errors:          []error{},
			errChan:         make(chan error),
			IsShutdown:      false,
			errorOnShutdown: true,
		},
		errCh:  errCh,
		logger: log,
	}, errCh
}

func NewTestConsumerWithErrors() (Consumer, chan error) {
	errCh := make(chan error, len(expectedErrors))
	log := logger.NewUPPLogger("test", "INFO")

	errChan := make(chan error, len(expectedErrors))
	for _, e := range expectedErrors {
		errChan <- e
	}

	return &messageConsumer{
		topics:                  []string{"topic"},
		consumerGroup:           "group",
		brokersConnectionString: "node",
		consumer: &MockConsumerGroup{
			messages:   messages,
			errors:     expectedErrors,
			IsShutdown: false,
			errChan:    errChan,
		},
		errCh:  errCh,
		logger: log,
	}, errCh
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
	}
}

func TestErrorDuringShutdown(t *testing.T) {
	consumer, errCh := NewTestConsumerWithErrChan()
	defer close(errCh)

	consumer.Shutdown()

	var actualError error
	select {
	case actualError = <-errCh:
		assert.NotNil(t, actualError, "Was expecting non-nil error on consumer shutdown")
	default:
		assert.NotNil(t, actualError, "Was expecting error on consumer shutdown")
	}
}

func TestMessageConsumer_StartListeningConsumerErrors(t *testing.T) {
	var count int32
	consumer, errChan := NewTestConsumerWithErrors()
	defer close(errChan)

	var actualErrors []error
	stopChan := make(chan struct{})
	stoppedChan := make(chan struct{})

	go func() {
		defer close(stoppedChan)
		for {
			select {
			case actualError := <-errChan:
				actualErrors = append(actualErrors, actualError)
			case <-stopChan:
				return
			}
		}
	}()

	go func() {
		consumer.StartListening(func(msg FTMessage) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
	}()

	time.Sleep(1 * time.Second)

	close(stopChan)
	<-stoppedChan
	assert.Equal(t, int32(len(messages)), atomic.LoadInt32(&count), "Handler wasn't called the expected number of times.")
	assert.Equal(t, expectedErrors, actualErrors, "Didn't get the expected errors from the consumer.")
}

func TestMessageConsumer_StartListeningHandlerErrors(t *testing.T) {
	var count int32
	consumer, errChan := NewTestConsumerWithErrChan()
	defer close(errChan)

	var actualErrors []error
	stopChan := make(chan struct{})
	stoppedChan := make(chan struct{})
	go func() {
		defer close(stoppedChan)
		for {
			select {
			case actualError := <-errChan:
				actualErrors = append(actualErrors, actualError)
			case <-stopChan:
				return
			}
		}
	}()

	go func() {
		consumer.StartListening(func(msg FTMessage) error {
			atomic.AddInt32(&count, 1)
			index := atomic.LoadInt32(&count) - 1

			if index < int32(len(expectedErrors)) {
				return expectedErrors[index]
			}

			return nil
		})
	}()

	time.Sleep(1 * time.Second)

	close(stopChan)
	<-stoppedChan

	assert.Equal(t, int32(len(messages)), atomic.LoadInt32(&count))
	assert.Equal(t, expectedErrors, actualErrors, "Didn't get the expected errors from the consumer handler.")
}

func TestMessageConsumer_StartListening(t *testing.T) {
	var count int32
	consumer := NewTestConsumer()
	go func() {
		consumer.StartListening(func(msg FTMessage) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
	}()
	time.Sleep(1 * time.Second)
	assert.Equal(t, int32(len(messages)), atomic.LoadInt32(&count))
}

func TestMessageConsumerContinuesWhenHandlerReturnsError(t *testing.T) {
	var count int32
	consumer := NewTestConsumer()
	go func() {
		consumer.StartListening(func(msg FTMessage) error {
			atomic.AddInt32(&count, 1)
			return errors.New("test error")
		})
	}()
	time.Sleep(1 * time.Second)
	assert.Equal(t, int32(len(messages)), atomic.LoadInt32(&count))
}

func TestPerseverantConsumerListensToConsumer(t *testing.T) {
	var count int32
	consumer := perseverantConsumer{consumer: NewTestConsumer()}

	go func() {
		consumer.StartListening(func(msg FTMessage) error {
			atomic.AddInt32(&count, 1)
			return nil
		})
	}()

	time.Sleep(1 * time.Second)
	assert.Equal(t, int32(len(messages)), atomic.LoadInt32(&count))

	consumer.Shutdown()
}
