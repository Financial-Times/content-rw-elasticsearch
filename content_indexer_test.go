package main

import (
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/olivere/elastic.v2"
	"io/ioutil"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type esServiceMock struct {
	mock.Mock
}

func (*esServiceMock) getSchemaHealth() (string, error) {
	panic("implement me")
}

func (service *esServiceMock) writeData(conceptType string, uuid string, payload interface{}) (*elastic.IndexResult, error) {
	args := service.Called(conceptType, uuid, payload)
	return args.Get(0).(*elastic.IndexResult), args.Error(1)
}

func (service *esServiceMock) deleteData(conceptType string, uuid string) (*elastic.DeleteResult, error) {
	args := service.Called(conceptType, uuid)
	return args.Get(0).(*elastic.DeleteResult), args.Error(1)
}

func (service *esServiceMock) setClient(client esClientI) {

}

func (service *esServiceMock) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	args := service.Called()
	return args.Get(0).(*elastic.ClusterHealthResponse), args.Error(1)
}

type elasticClientMock struct {
	mock.Mock
}

func (client elasticClientMock) IndexGet() *elastic.IndicesGetService {
	args := client.Called()
	return args.Get(0).(*elastic.IndicesGetService)
}

func (client elasticClientMock) ClusterHealth() *elastic.ClusterHealthService {
	args := client.Called()
	return args.Get(0).(*elastic.ClusterHealthService)
}

func (client elasticClientMock) Index() *elastic.IndexService {
	args := client.Called()
	return args.Get(0).(*elastic.IndexService)
}

func (client elasticClientMock) Get() *elastic.GetService {
	args := client.Called()
	return args.Get(0).(*elastic.GetService)
}

func (client elasticClientMock) Delete() *elastic.DeleteService {
	args := client.Called()
	return args.Get(0).(*elastic.DeleteService)
}

func (client elasticClientMock) PerformRequest(method, path string, params url.Values, body interface{}, ignoreErrors ...int) (*elastic.Response, error) {
	args := client.Called()
	return args.Get(0).(*elastic.Response), args.Error(1)
}

type logHook struct {
	sync.Mutex
	Entries []*logrus.Entry
}

func (hook *logHook) Fire(e *logrus.Entry) error {
	hook.Lock()
	defer hook.Unlock()
	hook.Entries = append(hook.Entries, e)
	return nil
}

func (hook *logHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *logHook) LastEntry() (l *logrus.Entry) {
	hook.Lock()
	defer hook.Unlock()
	if i := len(hook.Entries) - 1; i >= 0 {
		return hook.Entries[i]
	}
	return nil
}

// Reset removes all Entries from this test hook.
func (hook *logHook) Reset() {
	hook.Entries = make([]*logrus.Entry, 0)
}

func TestStartClient(t *testing.T) {
	assert := assert.New(t)

	accessConfig := esAccessConfig{
		accessKey:  "key",
		secretKey:  "secret",
		esEndpoint: "endpoint",
	}

	queueConfig := consumer.QueueConfig{
		Addrs:                []string{"address"},
		Group:                "group",
		Topic:                "topic",
		Queue:                "queue",
		ConcurrentProcessing: false,
	}

	newAmazonClient = func(config esAccessConfig) (esClientI, error) {
		return &elasticClientMock{}, nil
	}

	indexer := contentIndexer{}

	indexer.start("app", "name", "index", "1984", accessConfig, queueConfig)
	defer indexer.stop()

	time.Sleep(100 * time.Millisecond)

	assert.NotNil(indexer.esServiceInstance, "Elastic Service should be initialized")
	assert.Equal("index", (indexer.esServiceInstance).(*esService).indexName, "Wrong index")
	(indexer.esServiceInstance).(*esService).Lock()
	assert.NotNil((indexer.esServiceInstance).(*esService).elasticClient, "Elastic client should be initialized")
	(indexer.esServiceInstance).(*esService).Unlock()
}

func TestStartClientError(t *testing.T) {
	assert := assert.New(t)

	hook := &logHook{}
	logrus.AddHook(hook)

	accessConfig := esAccessConfig{
		accessKey:  "key",
		secretKey:  "secret",
		esEndpoint: "endpoint",
	}

	queueConfig := consumer.QueueConfig{
		Addrs:                []string{"address"},
		Group:                "group",
		Topic:                "topic",
		Queue:                "queue",
		ConcurrentProcessing: false,
	}

	newAmazonClient = func(config esAccessConfig) (esClientI, error) {
		return nil, elastic.ErrNoClient
	}

	indexer := contentIndexer{}

	indexer.start("app", "name", "index", "1984", accessConfig, queueConfig)
	defer indexer.stop()

	time.Sleep(100 * time.Millisecond)

	assert.Equal("error", hook.LastEntry().Level.String(), "Wrong log")
	assert.NotNil(indexer.esServiceInstance, "Elastic Service should be initialized")
	assert.Equal("index", (indexer.esServiceInstance).(*esService).indexName, "Wrong index")
	assert.Nil((indexer.esServiceInstance).(*esService).elasticClient, "Elastic client should not be initialized")
}

func TestHandleWriteMessage(t *testing.T) {
	assert := assert.New(t)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}

	serviceMock.On("writeData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)

	indexer := contentIndexer{esServiceInstance: serviceMock}
	indexer.handleMessage(consumer.Message{Body: string(inputJSON)})

	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageBlog(t *testing.T) {
	assert := assert.New(t)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")
	input := strings.Replace(string(inputJSON), "FTCOM-METHODE", "FT-LABS-WP1234", 1)

	serviceMock := &esServiceMock{}

	serviceMock.On("writeData", "FTBlogs", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)

	indexer := contentIndexer{esServiceInstance: serviceMock}
	indexer.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageBlogWithHeader(t *testing.T) {
	assert := assert.New(t)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")
	input := strings.Replace(string(inputJSON), "FTCOM-METHODE", "invalid", 1)

	serviceMock := &esServiceMock{}

	serviceMock.On("writeData", "FTBlogs", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)

	indexer := contentIndexer{esServiceInstance: serviceMock}
	indexer.handleMessage(consumer.Message{Body: input, Headers: map[string]string{"Origin-System-Id": "wordpress"}})

	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageVideo(t *testing.T) {
	assert := assert.New(t)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")
	input := strings.Replace(string(inputJSON), "FTCOM-METHODE", "NEXT-VIDEO-EDITOR", 1)

	serviceMock := &esServiceMock{}

	serviceMock.On("writeData", "FTVideos", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)

	indexer := contentIndexer{esServiceInstance: serviceMock}
	indexer.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageUnknownType(t *testing.T) {
	assert := assert.New(t)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")
	input := strings.Replace(string(inputJSON), `"Article"`, `"Content"`, 1)

	serviceMock := &esServiceMock{}

	indexer := contentIndexer{esServiceInstance: serviceMock}
	indexer.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertNotCalled(t, "writeData", mock.Anything, "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything)
	serviceMock.AssertNotCalled(t, "deleteData", mock.Anything, "aae9611e-f66c-4fe4-a6c6-2e2bdea69060")
	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageNoType(t *testing.T) {
	assert := assert.New(t)

	hook := &logHook{}
	logrus.AddHook(hook)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")
	input := strings.Replace(string(inputJSON), "FTCOM-METHODE", "invalid", 1)

	serviceMock := &esServiceMock{}

	indexer := contentIndexer{esServiceInstance: serviceMock}
	indexer.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertNotCalled(t, "writeData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "deleteData", mock.Anything, mock.Anything)
	assert.Equal("error", hook.LastEntry().Level.String(), "Wrong log")
}

func TestHandleWriteMessageError(t *testing.T) {
	assert := assert.New(t)

	hook := &logHook{}
	logrus.AddHook(hook)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}

	serviceMock.On("writeData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, elastic.ErrTimeout)

	indexer := contentIndexer{esServiceInstance: serviceMock}
	indexer.handleMessage(consumer.Message{Body: string(inputJSON)})

	serviceMock.AssertExpectations(t)
	assert.Equal("error", hook.LastEntry().Level.String(), "Wrong log")
}

func TestHandleDeleteMessage(t *testing.T) {
	assert := assert.New(t)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")
	input := strings.Replace(string(inputJSON), `"marked_deleted": false`, `"marked_deleted": true`, 1)

	serviceMock := &esServiceMock{}

	serviceMock.On("deleteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060").Return(&elastic.DeleteResult{}, nil)

	indexer := contentIndexer{esServiceInstance: serviceMock}
	indexer.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertExpectations(t)
}

func TestHandleDeleteMessageError(t *testing.T) {
	assert := assert.New(t)

	hook := &logHook{}
	logrus.AddHook(hook)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")
	input := strings.Replace(string(inputJSON), `"marked_deleted": false`, `"marked_deleted": true`, 1)

	serviceMock := &esServiceMock{}

	serviceMock.On("deleteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060").Return(&elastic.DeleteResult{}, elastic.ErrTimeout)

	indexer := contentIndexer{esServiceInstance: serviceMock}
	indexer.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertExpectations(t)
	assert.Equal("error", hook.LastEntry().Level.String(), "Wrong log")
}

func TestHandleMessageJsonError(t *testing.T) {
	assert := assert.New(t)

	hook := &logHook{}
	logrus.AddHook(hook)

	serviceMock := &esServiceMock{}

	indexer := &contentIndexer{esServiceInstance: serviceMock}
	indexer.handleMessage(consumer.Message{Body: "malformed json"})

	assert.Equal("error", hook.LastEntry().Level.String(), "Wrong log")
	serviceMock.AssertNotCalled(t, "writeData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "deleteData", mock.Anything, mock.Anything)
}

func TestHandleSyntheticMessage(t *testing.T) {
	assert := assert.New(t)

	hook := &logHook{}
	logrus.AddHook(hook)

	serviceMock := &esServiceMock{}
	indexer := &contentIndexer{esServiceInstance: serviceMock}
	indexer.handleMessage(consumer.Message{Headers: map[string]string{"X-Request-Id": "SYNTHETIC-REQ-MON_WuLjbRpCgh"}})

	assert.Equal("info", hook.LastEntry().Level.String(), "Wrong log")
	serviceMock.AssertNotCalled(t, "writeData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "deleteData", mock.Anything, mock.Anything)
}
