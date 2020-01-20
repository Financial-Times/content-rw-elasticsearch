package service

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/es"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/service/concept"
	logTest "github.com/Financial-Times/go-logger/test"
	logger2 "github.com/Financial-Times/go-logger/v2"
	consumer "github.com/Financial-Times/message-queue-gonsumer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gopkg.in/olivere/elastic.v2"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/content"
)

type esServiceMock struct {
	mock.Mock
}

func (*esServiceMock) GetSchemaHealth() (string, error) {
	panic("implement me")
}

func (service *esServiceMock) WriteData(conceptType string, uuid string, payload interface{}) (*elastic.IndexResult, error) {
	args := service.Called(conceptType, uuid, payload)
	return args.Get(0).(*elastic.IndexResult), args.Error(1)
}

func (service *esServiceMock) DeleteData(conceptType string, uuid string) (*elastic.DeleteResult, error) {
	args := service.Called(conceptType, uuid)
	return args.Get(0).(*elastic.DeleteResult), args.Error(1)
}

func (service *esServiceMock) SetClient(client es.ClientI) {

}

func (service *esServiceMock) GetClusterHealth() (*elastic.ClusterHealthResponse, error) {
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

type concordanceApiMock struct {
	mock.Mock
}

func (m *concordanceApiMock) GetConcepts(tid string, ids []string) (map[string]concept.ConceptModel, error) {
	args := m.Called(tid, ids)
	return args.Get(0).(map[string]concept.ConceptModel), args.Error(1)
}

func TestStartClient(t *testing.T) {
	assert := assert.New(t)

	accessConfig := es.AccessConfig{
		AccessKey: "key",
		SecretKey: "secret",
		Endpoint:  "endpoint",
	}

	queueConfig := consumer.QueueConfig{
		Addrs:                []string{"address"},
		Group:                "group",
		Topic:                "topic",
		Queue:                "queue",
		ConcurrentProcessing: false,
	}

	var NewClient = func(config es.AccessConfig, c *http.Client) (es.ClientI, error) {
		return &elasticClientMock{}, nil
	}
	concordanceApiMock := new(concordanceApiMock)

	var wg sync.WaitGroup
	logConf := logger2.KeyNamesConfig{KeyTime: "@time"}
	l := logger2.NewUPPLogger("Test", "PANIC", logConf)
	handler := NewMessageHandler(es.NewService("index"), concordanceApiMock, http.DefaultClient, queueConfig, &wg, NewClient, l)

	handler.Start("http://api.ft.com/", accessConfig, http.DefaultClient)
	defer handler.Stop()

	time.Sleep(100 * time.Millisecond)

	assert.NotNil(handler.esService, "Elastic Service should be initialized")
	assert.Equal("index", (handler.esService).(*es.Service).IndexName, "Wrong index")
	(handler.esService).(*es.Service).Lock()
	assert.NotNil((handler.esService).(*es.Service).ElasticClient, "Elastic client should be initialized")
	(handler.esService).(*es.Service).Unlock()
}

func TestStartClientError(t *testing.T) {
	assert := assert.New(t)

	hook := logTest.NewTestHook("content-rw-elasticsearch")

	accessConfig := es.AccessConfig{
		AccessKey: "key",
		SecretKey: "secret",
		Endpoint:  "endpoint",
	}

	queueConfig := consumer.QueueConfig{
		Addrs:                []string{"address"},
		Group:                "group",
		Topic:                "topic",
		Queue:                "queue",
		ConcurrentProcessing: false,
	}

	var NewClient = func(config es.AccessConfig, c *http.Client) (es.ClientI, error) {
		return nil, elastic.ErrNoClient
	}

	concordanceApiMock := new(concordanceApiMock)

	var wg sync.WaitGroup
	logConf := logger2.KeyNamesConfig{KeyTime: "@time"}
	l := logger2.NewUPPLogger("Test", "PANIC", logConf)
	handler := NewMessageHandler(es.NewService("index"), concordanceApiMock, http.DefaultClient, queueConfig, &wg, NewClient, l)

	handler.Start("http://api.ft.com/", accessConfig, http.DefaultClient)
	defer handler.Stop()

	time.Sleep(100 * time.Millisecond)

	require.NotNil(t, hook.LastEntry())
	assert.Equal("error", hook.LastEntry().Level.String(), "Wrong log")
	assert.NotNil(handler.esService, "Elastic Service should be initialized")
	assert.Equal("index", (handler.esService).(*es.Service).IndexName, "Wrong index")
	assert.Nil((handler.esService).(*es.Service).ElasticClient, "Elastic client should not be initialized")
}

func TestHandleWriteMessage(t *testing.T) {
	assert := assert.New(t)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceApiMock := new(concordanceApiMock)
	concordanceApiMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.ConceptModel{}, nil)

	handler := MessageHandler{esService: serviceMock, ConceptGetter: concordanceApiMock}
	handler.handleMessage(consumer.Message{Body: string(inputJSON)})

	assert.Equal(1, len(serviceMock.Calls))

	data := serviceMock.Calls[0].Arguments.Get(2)
	model, ok := data.(content.IndexModel)
	if !ok {
		assert.Fail("Result is not content.IndexModel")
	}
	assert.NotEmpty(model.Body)

	serviceMock.AssertExpectations(t)
	concordanceApiMock.AssertExpectations(t)
}

func TestHandleWriteMessageFromBodyXML(t *testing.T) {
	assert := assert.New(t)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModelWithBodyXML.json")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceApiMock := new(concordanceApiMock)
	concordanceApiMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.ConceptModel{}, nil)

	handler := MessageHandler{esService: serviceMock, ConceptGetter: concordanceApiMock}
	handler.handleMessage(consumer.Message{Body: string(inputJSON)})

	assert.Equal(1, len(serviceMock.Calls))

	data := serviceMock.Calls[0].Arguments.Get(2)
	model, ok := data.(content.IndexModel)
	if !ok {
		assert.Fail("Result is not content.IndexModel")
	}
	assert.NotEmpty(model.Body)

	serviceMock.AssertExpectations(t)
	concordanceApiMock.AssertExpectations(t)
}

func TestHandleWriteMessageBlog(t *testing.T) {
	assert := assert.New(t)

	input, err := modifyTestInputAuthority("FT-LABS-WP1234")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTBlogs", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceApiMock := new(concordanceApiMock)
	concordanceApiMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.ConceptModel{}, nil)

	handler := MessageHandler{esService: serviceMock, ConceptGetter: concordanceApiMock}
	handler.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertExpectations(t)
	concordanceApiMock.AssertExpectations(t)
}

func TestHandleWriteMessageBlogWithHeader(t *testing.T) {
	assert := assert.New(t)

	input, err := modifyTestInputAuthority("invalid")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTBlogs", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceApiMock := new(concordanceApiMock)
	concordanceApiMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.ConceptModel{}, nil)

	handler := MessageHandler{esService: serviceMock, ConceptGetter: concordanceApiMock}
	handler.handleMessage(consumer.Message{Body: input, Headers: map[string]string{"Origin-System-Id": "wordpress", "Content-Type": "application/json"}})

	serviceMock.AssertExpectations(t)
	concordanceApiMock.AssertExpectations(t)
}

func TestHandleWriteMessageVideo(t *testing.T) {
	assert := assert.New(t)

	input, err := modifyTestInputAuthority("NEXT-VIDEO-EDITOR")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTVideos", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceApiMock := new(concordanceApiMock)
	concordanceApiMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.ConceptModel{}, nil)

	handler := MessageHandler{esService: serviceMock, ConceptGetter: concordanceApiMock}
	handler.handleMessage(consumer.Message{Body: input, Headers: map[string]string{"Content-Type": "application/json"}})

	serviceMock.AssertExpectations(t)
	concordanceApiMock.AssertExpectations(t)
}

func TestHandleWriteMessageAudio(t *testing.T) {
	assert := assert.New(t)

	input, err := modifyTestInputAuthority("NEXT-VIDEO-EDITOR")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTAudios", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceApiMock := new(concordanceApiMock)
	concordanceApiMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.ConceptModel{}, nil)

	handler := MessageHandler{esService: serviceMock, ConceptGetter: concordanceApiMock}
	handler.handleMessage(consumer.Message{Body: input, Headers: map[string]string{"Content-Type": "vnd.ft-upp-audio+json"}})

	serviceMock.AssertExpectations(t)
	concordanceApiMock.AssertExpectations(t)
}

func TestHandleWriteMessageArticleByHeaderType(t *testing.T) {
	assert := assert.New(t)

	input, err := modifyTestInputAuthority("invalid")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceApiMock := new(concordanceApiMock)
	concordanceApiMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.ConceptModel{}, nil)

	handler := MessageHandler{esService: serviceMock, ConceptGetter: concordanceApiMock}
	handler.handleMessage(consumer.Message{Body: input, Headers: map[string]string{"Content-Type": "application/vnd.ft-upp-article"}})

	serviceMock.AssertExpectations(t)
	concordanceApiMock.AssertExpectations(t)
}

func TestHandleWriteMessageUnknownType(t *testing.T) {
	assert := assert.New(t)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")
	input := strings.Replace(string(inputJSON), `"Article"`, `"Content"`, 1)

	serviceMock := &esServiceMock{}

	handler := MessageHandler{esService: serviceMock}
	handler.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, "aae9611e-f66c-4fe4-a6c6-2e2bdea69060")
	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageNoUUIDForMetadataPublish(t *testing.T) {
	assert := assert.New(t)

	hook := logTest.NewTestHook("content-rw-elasticsearch")

	inputJSON, err := ioutil.ReadFile("testdata/testEnrichedContentModel3.json")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}

	handler := MessageHandler{esService: serviceMock}
	handler.handleMessage(consumer.Message{Body: string(inputJSON), Headers: map[string]string{originHeader: methodeOrigin}})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, "b17756fe-0f62-4cf1-9deb-ca7a2ff80172", mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, "b17756fe-0f62-4cf1-9deb-ca7a2ff80172")
	serviceMock.AssertExpectations(t)

	require.NotNil(t, hook.LastEntry())
	assert.Equal("info", hook.LastEntry().Level.String(), "Wrong log")
}

func TestHandleWriteMessageNoType(t *testing.T) {
	assert := assert.New(t)

	hook := logTest.NewTestHook("content-rw-elasticsearch")

	input, err := modifyTestInputAuthority("invalid")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}

	handler := MessageHandler{esService: serviceMock}
	handler.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, mock.Anything)
	require.NotNil(t, hook.LastEntry())
	assert.Equal("error", hook.LastEntry().Level.String(), "Wrong log")
}

func TestHandleWriteMessageError(t *testing.T) {
	assert := assert.New(t)

	hook := logTest.NewTestHook("content-rw-elasticsearch")

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, elastic.ErrTimeout)
	concordanceApiMock := new(concordanceApiMock)
	concordanceApiMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.ConceptModel{}, nil)

	handler := MessageHandler{esService: serviceMock, ConceptGetter: concordanceApiMock}
	handler.handleMessage(consumer.Message{Body: string(inputJSON)})

	serviceMock.AssertExpectations(t)
	require.NotNil(t, hook.LastEntry())
	assert.Equal("error", hook.LastEntry().Level.String(), "Wrong log")

	concordanceApiMock.AssertExpectations(t)
}

func TestHandleDeleteMessage(t *testing.T) {
	assert := assert.New(t)

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")
	input := strings.Replace(string(inputJSON), `"markedDeleted": "false"`, `"markedDeleted": "true"`, 1)

	serviceMock := &esServiceMock{}
	serviceMock.On("DeleteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060").Return(&elastic.DeleteResult{}, nil)

	handler := MessageHandler{esService: serviceMock}
	handler.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertExpectations(t)
}

func TestHandleDeleteMessageError(t *testing.T) {
	assert := assert.New(t)

	hook := logTest.NewTestHook("content-rw-elasticsearch")

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	assert.NoError(err, "Unexpected error")
	input := strings.Replace(string(inputJSON), `"markedDeleted": "false"`, `"markedDeleted": "true"`, 1)

	serviceMock := &esServiceMock{}

	serviceMock.On("DeleteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060").Return(&elastic.DeleteResult{}, elastic.ErrTimeout)

	handler := MessageHandler{esService: serviceMock}
	handler.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertExpectations(t)
	require.NotNil(t, hook.LastEntry())
	assert.Equal("error", hook.LastEntry().Level.String(), "Wrong log")
}

func TestHandleMessageJsonError(t *testing.T) {
	assert := assert.New(t)

	hook := logTest.NewTestHook("content-rw-elasticsearch")

	serviceMock := &esServiceMock{}

	handler := &MessageHandler{esService: serviceMock}
	handler.handleMessage(consumer.Message{Body: "malformed json"})

	require.NotNil(t, hook.LastEntry())
	assert.Equal("error", hook.LastEntry().Level.String(), "Wrong log")
	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, mock.Anything)
}

func TestHandleSyntheticMessage(t *testing.T) {
	assert := assert.New(t)

	hook := logTest.NewTestHook("content-rw-elasticsearch")

	serviceMock := &esServiceMock{}
	handler := &MessageHandler{esService: serviceMock}
	handler.handleMessage(consumer.Message{Headers: map[string]string{"X-Request-Id": "SYNTHETIC-REQ-MON_WuLjbRpCgh"}})

	require.NotNil(t, hook.LastEntry())
	assert.Equal("info", hook.LastEntry().Level.String(), "Wrong log")
	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, mock.Anything)
}

func TestHandlePACMessage(t *testing.T) {
	assert := assert.New(t)

	hook := logTest.NewTestHook("content-rw-elasticsearch")

	serviceMock := &esServiceMock{}
	handler := &MessageHandler{esService: serviceMock}
	handler.handleMessage(consumer.Message{Headers: map[string]string{"Origin-System-Id": "http://cmdb.ft.com/systems/pac"}, Body: "{}"})

	require.NotNil(t, hook.LastEntry())
	assert.Equal("info", hook.LastEntry().Level.String(), "Wrong log")
	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, mock.Anything)
}

func modifyTestInputAuthority(replacement string) (string, error) {

	inputJSON, err := ioutil.ReadFile("testdata/exampleEnrichedContentModel.json")
	input := strings.Replace(string(inputJSON), "FTCOM-METHODE", replacement, 1)
	return input, err
}
