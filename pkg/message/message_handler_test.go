package message

import (
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/concept"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/config"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/es"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/mapper"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/schema"
	tst "github.com/Financial-Times/content-rw-elasticsearch/v2/test"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/olivere/elastic.v2"
)

type esServiceMock struct {
	mock.Mock
}

func (*esServiceMock) GetSchemaHealth() (string, error) {
	panic("implement me")
}

func (s *esServiceMock) WriteData(conceptType string, uuid string, payload interface{}) (*elastic.IndexResult, error) {
	args := s.Called(conceptType, uuid, payload)
	return args.Get(0).(*elastic.IndexResult), args.Error(1)
}

func (s *esServiceMock) DeleteData(conceptType string, uuid string) (*elastic.DeleteResult, error) {
	args := s.Called(conceptType, uuid)
	return args.Get(0).(*elastic.DeleteResult), args.Error(1)
}

func (s *esServiceMock) SetClient(client es.Client) {

}

func (s *esServiceMock) GetClusterHealth() (*elastic.ClusterHealthResponse, error) {
	args := s.Called()
	return args.Get(0).(*elastic.ClusterHealthResponse), args.Error(1)
}

type elasticClientMock struct {
	mock.Mock
}

func (c *elasticClientMock) IndexGet() *elastic.IndicesGetService {
	args := c.Called()
	return args.Get(0).(*elastic.IndicesGetService)
}

func (c *elasticClientMock) ClusterHealth() *elastic.ClusterHealthService {
	args := c.Called()
	return args.Get(0).(*elastic.ClusterHealthService)
}

func (c *elasticClientMock) Index() *elastic.IndexService {
	args := c.Called()
	return args.Get(0).(*elastic.IndexService)
}

func (c *elasticClientMock) Get() *elastic.GetService {
	args := c.Called()
	return args.Get(0).(*elastic.GetService)
}

func (c *elasticClientMock) Delete() *elastic.DeleteService {
	args := c.Called()
	return args.Get(0).(*elastic.DeleteService)
}

func (c *elasticClientMock) PerformRequest(method, path string, params url.Values, body interface{}, ignoreErrors ...int) (*elastic.Response, error) {
	args := c.Called()
	return args.Get(0).(*elastic.Response), args.Error(1)
}

type concordanceAPIMock struct {
	mock.Mock
}

var defaultESClient = func(config es.AccessConfig, c *http.Client, log *logger.UPPLogger) (es.Client, error) {
	return &elasticClientMock{}, nil
}

var errorESClient = func(config es.AccessConfig, c *http.Client, log *logger.UPPLogger) (es.Client, error) {
	return nil, elastic.ErrNoClient
}

func (m *concordanceAPIMock) GetConcepts(tid string, ids []string) (map[string]concept.Model, error) {
	args := m.Called(tid, ids)
	return args.Get(0).(map[string]concept.Model), args.Error(1)
}

func mockMessageHandler(esClient ESClient, mocks ...interface{}) (es.AccessConfig, *Handler) {
	uppLogger := logger.NewUPPLogger(config.AppName, config.AppDefaultLogLevel)

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

	concordanceAPI := new(concordanceAPIMock)
	esService := new(esServiceMock)
	for _, m := range mocks {
		switch m.(type) {
		case *concordanceAPIMock:
			concordanceAPI = m.(*concordanceAPIMock)
		case *esServiceMock:
			esService = m.(*esServiceMock)
		}
	}

	mapperHandler := mockMapperHandler(concordanceAPI, uppLogger)

	handler := NewMessageHandler(esService, mapperHandler, http.DefaultClient, queueConfig, esClient, uppLogger)
	if mocks == nil {
		handler = NewMessageHandler(es.NewService("index"), mapperHandler, http.DefaultClient, queueConfig, esClient, uppLogger)
	}
	return accessConfig, handler
}

func mockMapperHandler(concordanceAPIMock *concordanceAPIMock, log *logger.UPPLogger) *mapper.Handler {
	appConfig := initAppConfig()
	mapperHandler := mapper.NewMapperHandler(concordanceAPIMock, "http://api.ft.com", appConfig, log)
	return mapperHandler
}

func initAppConfig() config.AppConfig {
	appConfig, err := config.ParseConfig("app.yml")
	if err != nil {
		log.Fatal(err)
	}
	return appConfig
}

func TestStartClient(t *testing.T) {
	expect := assert.New(t)

	accessConfig, handler := mockMessageHandler(defaultESClient)

	handler.Start("http://api.ft.com/", accessConfig)
	defer handler.Stop()

	time.Sleep(100 * time.Millisecond)

	expect.NotNil(handler.esService, "Elastic Service should be initialized")
	expect.Equal("index", handler.esService.(*es.ElasticsearchService).IndexName, "Wrong index")
	expect.NotNil(handler.esService.(*es.ElasticsearchService).GetClient(), "Elastic client should be initialized")
}
func TestStartClientError(t *testing.T) {
	expect := assert.New(t)

	accessConfig, handler := mockMessageHandler(errorESClient)

	handler.Start("http://api.ft.com/", accessConfig)
	defer handler.Stop()

	time.Sleep(100 * time.Millisecond)

	expect.NotNil(handler.esService, "Elastic Service should be initialized")
	expect.Equal("index", handler.esService.(*es.ElasticsearchService).IndexName, "Wrong index")
	expect.Nil(handler.esService.(*es.ElasticsearchService).ElasticClient, "Elastic client should not be initialized")
}

func TestHandleWriteMessage(t *testing.T) {
	expect := assert.New(t)

	inputJSON := tst.ReadTestResource("exampleEnrichedContentModel.json")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(consumer.Message{Body: string(inputJSON)})

	expect.Equal(1, len(serviceMock.Calls))

	data := serviceMock.Calls[0].Arguments.Get(2)
	model, ok := data.(schema.IndexModel)
	if !ok {
		expect.Fail("Result is not content.IndexModel")
	}
	expect.NotEmpty(model.Body)

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageFromBodyXML(t *testing.T) {
	expect := assert.New(t)

	inputJSON := tst.ReadTestResource("exampleEnrichedContentModelWithBodyXML.json")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(consumer.Message{Body: string(inputJSON)})

	expect.Equal(1, len(serviceMock.Calls))

	data := serviceMock.Calls[0].Arguments.Get(2)
	model, ok := data.(schema.IndexModel)
	if !ok {
		expect.Fail("Result is not content.IndexModel")
	}
	expect.NotEmpty(model.Body)

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageBlog(t *testing.T) {
	input := modifyTestInputAuthority("FT-LABS-WP1234")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTBlogs", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageBlogWithHeader(t *testing.T) {
	input := modifyTestInputAuthority("invalid")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTBlogs", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(consumer.Message{Body: input, Headers: map[string]string{"Origin-System-Id": "wordpress", "Content-Type": "application/json"}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageVideo(t *testing.T) {
	input := modifyTestInputAuthority("NEXT-VIDEO-EDITOR")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTVideos", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(consumer.Message{Body: input, Headers: map[string]string{"Content-Type": "application/json"}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageAudio(t *testing.T) {
	input := modifyTestInputAuthority("NEXT-VIDEO-EDITOR")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTAudios", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(consumer.Message{Body: input, Headers: map[string]string{"Content-Type": "vnd.ft-upp-audio+json"}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageArticleByHeaderType(t *testing.T) {
	input := modifyTestInputAuthority("invalid")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(consumer.Message{Body: input, Headers: map[string]string{"Content-Type": "application/vnd.ft-upp-article"}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageUnknownType(t *testing.T) {
	inputJSON := tst.ReadTestResource("exampleEnrichedContentModel.json")

	input := strings.Replace(string(inputJSON), `"Article"`, `"Content"`, 1)

	serviceMock := &esServiceMock{}

	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, "aae9611e-f66c-4fe4-a6c6-2e2bdea69060")
	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageNoUUIDForMetadataPublish(t *testing.T) {
	inputJSON := tst.ReadTestResource("testEnrichedContentModel3.json")

	serviceMock := &esServiceMock{}

	_, h := mockMessageHandler(defaultESClient, serviceMock)
	methodeOrigin := h.Mapper.Config.ContentMetadataMap.Get("methode").Origin
	h.handleMessage(consumer.Message{
		Body: string(inputJSON),
		Headers: map[string]string{
			originHeader: methodeOrigin,
		},
	})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, "b17756fe-0f62-4cf1-9deb-ca7a2ff80172", mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, "b17756fe-0f62-4cf1-9deb-ca7a2ff80172")
	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageNoType(t *testing.T) {
	input := modifyTestInputAuthority("invalid")

	serviceMock := &esServiceMock{}

	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, mock.Anything)
}

func TestHandleWriteMessageError(t *testing.T) {
	inputJSON := tst.ReadTestResource("exampleEnrichedContentModel.json")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, elastic.ErrTimeout)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(consumer.Message{Body: string(inputJSON)})

	serviceMock.AssertExpectations(t)

	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleDeleteMessage(t *testing.T) {
	inputJSON := tst.ReadTestResource("exampleEnrichedContentModel.json")
	input := strings.Replace(string(inputJSON), `"markedDeleted": "false"`, `"markedDeleted": "true"`, 1)

	serviceMock := &esServiceMock{}
	serviceMock.On("DeleteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060").Return(&elastic.DeleteResult{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertExpectations(t)
}

func TestHandleDeleteMessageError(t *testing.T) {
	inputJSON := tst.ReadTestResource("exampleEnrichedContentModel.json")
	input := strings.Replace(string(inputJSON), `"markedDeleted": "false"`, `"markedDeleted": "true"`, 1)

	serviceMock := &esServiceMock{}

	serviceMock.On("DeleteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060").Return(&elastic.DeleteResult{}, elastic.ErrTimeout)

	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(consumer.Message{Body: input})

	serviceMock.AssertExpectations(t)
}

func TestHandleMessageJsonError(t *testing.T) {
	serviceMock := &esServiceMock{}
	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(consumer.Message{Body: "malformed json"})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, mock.Anything)
}

func TestHandleSyntheticMessage(t *testing.T) {
	serviceMock := &esServiceMock{}
	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(consumer.Message{Headers: map[string]string{"X-Request-Id": "SYNTHETIC-REQ-MON_WuLjbRpCgh"}})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, mock.Anything)
}

func TestHandlePACMessage(t *testing.T) {
	serviceMock := &esServiceMock{}
	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(consumer.Message{Headers: map[string]string{"Origin-System-Id": config.PACOrigin}, Body: "{}"})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, mock.Anything)
}

func TestHandlePACMessageWithOldSparkContent(t *testing.T) {
	input := modifyTestInputAuthority("cct")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(consumer.Message{Body: input, Headers: map[string]string{originHeader: config.PACOrigin}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandlePACMessageWithSparkContent(t *testing.T) {
	input := modifyTestInputAuthority("spark")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(consumer.Message{Body: input, Headers: map[string]string{originHeader: config.PACOrigin}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func modifyTestInputAuthority(replacement string) string {
	inputJSON := tst.ReadTestResource("exampleEnrichedContentModel.json")
	input := strings.Replace(string(inputJSON), "FTCOM-METHODE", replacement, 1)
	return input
}
