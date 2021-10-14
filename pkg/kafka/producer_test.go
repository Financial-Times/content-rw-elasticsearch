package kafka

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	logger "github.com/Financial-Times/go-logger/v2"
	"github.com/Shopify/sarama/mocks"
	"github.com/stretchr/testify/assert"
)

const (
	testBrokers = "localhost:29092"
	testTopic   = "testTopic"
)

func NewTestProducer(t *testing.T, brokers string, topic string) (*messageProducer, error) {
	msp := mocks.NewSyncProducer(t, nil)
	brokerSlice := strings.Split(brokers, ",")

	msp.ExpectSendMessageAndSucceed()

	return &messageProducer{
		brokers:  brokerSlice,
		topic:    topic,
		producer: msp,
	}, nil
}

func TestNewProducer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test as it requires a connection to Kafka.")
	}

	log := logger.NewUPPLogger("test", "INFO")
	producer, err := newProducer(testBrokers, testTopic, DefaultProducerConfig(), log)

	assert.NoError(t, err)

	err = producer.connectivityCheck()
	assert.NoError(t, err)

	assert.Equal(t, 16777216, producer.config.Producer.MaxMessageBytes, "maximum message size using default config")
}

func TestNewProducerBadUrl(t *testing.T) {
	server := httptest.NewServer(nil)
	kURL := server.URL[strings.LastIndex(server.URL, "/")+1:]
	server.Close()

	log := logger.NewUPPLogger("test", "INFO")
	_, err := newProducer(kURL, testTopic, DefaultProducerConfig(), log)

	assert.Error(t, err)
}

func TestClient_SendMessage(t *testing.T) {
	kc, _ := NewTestProducer(t, testBrokers, testTopic)
	err := kc.sendMessage(NewFTMessage(nil, "Body"))

	assert.NoError(t, err)
}

func TestNewPerseverantProducer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test as it requires a connection to Kafka.")
	}

	log := logger.NewUPPLogger("test", "INFO")
	producer, err := NewPerseverantProducer(testBrokers, testTopic, nil, 0, time.Second, log)
	assert.NoError(t, err)

	time.Sleep(time.Second)

	err = producer.ConnectivityCheck()
	assert.NoError(t, err)
}

func TestNewPerseverantProducerNotConnected(t *testing.T) {
	server := httptest.NewServer(nil)
	kURL := server.URL[strings.LastIndex(server.URL, "/")+1:]
	server.Close()

	log := logger.NewUPPLogger("test", "INFO")
	producer, err := NewPerseverantProducer(kURL, testTopic, nil, 0, time.Second, log)
	assert.NoError(t, err)

	err = producer.ConnectivityCheck()
	assert.EqualError(t, err, errProducerNotConnected)
}

func TestNewPerseverantProducerWithInitialDelay(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test as it requires a connection to Kafka.")
	}

	log := logger.NewUPPLogger("test", "INFO")
	producer, err := NewPerseverantProducer(testBrokers, testTopic, nil, time.Second, time.Second, log)
	assert.NoError(t, err)

	err = producer.ConnectivityCheck()
	assert.EqualError(t, err, errProducerNotConnected)

	time.Sleep(2 * time.Second)
	err = producer.ConnectivityCheck()
	assert.NoError(t, err)
}

func TestPerseverantProducerForwardsToProducer(t *testing.T) {
	mp, _ := NewTestProducer(t, brokerURL, testTopic)
	p := PerseverantProducer{producer: mp}

	msg := FTMessage{
		Headers: map[string]string{
			"X-Request-Id": "test",
		},
		Body: `{"foo":"bar"}`,
	}

	actual := p.SendMessage(msg)
	assert.NoError(t, actual)

	err := p.Close()
	assert.NoError(t, err)
}

func TestPerseverantProducerNotConnectedCannotSendMessages(t *testing.T) {
	p := PerseverantProducer{}

	msg := FTMessage{
		Headers: map[string]string{
			"X-Request-Id": "test",
		},
		Body: `{"foo":"bar"}`,
	}

	actual := p.SendMessage(msg)
	assert.EqualError(t, actual, errProducerNotConnected)
}
