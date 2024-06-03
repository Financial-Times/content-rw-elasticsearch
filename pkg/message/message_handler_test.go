package message

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/concept"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/config"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/es"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/mapper"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/policy"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/schema"
	testdata "github.com/Financial-Times/content-rw-elasticsearch/v4/test"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockOpaAgent struct {
	returnResult *policy.ContentPolicyResult
	returnError  error
}

func (m mockOpaAgent) EvaluateContentPolicy(_ map[string]interface{}) (*policy.ContentPolicyResult, error) {
	return m.returnResult, m.returnError
}

type esServiceMock struct {
	mock.Mock
}

func (*esServiceMock) GetSchemaHealth(ctx context.Context) (string, error) {
	panic("implement me")
}

func (s *esServiceMock) WriteData(ctx context.Context, uuid string, payload interface{}) (*elastic.IndexResponse, error) {
	args := s.Called(uuid, payload)
	return args.Get(0).(*elastic.IndexResponse), args.Error(1)
}

func (s *esServiceMock) DeleteData(ctx context.Context, uuid string) (*elastic.DeleteResponse, error) {
	args := s.Called(uuid)
	return args.Get(0).(*elastic.DeleteResponse), args.Error(1)
}

func (s *esServiceMock) SetClient(client es.Client) {

}

func (s *esServiceMock) GetClusterHealth(ctx context.Context) (*elastic.ClusterHealthResponse, error) {
	args := s.Called()
	return args.Get(0).(*elastic.ClusterHealthResponse), args.Error(1)
}

type elasticClientMock struct {
	mock.Mock
}

func (c *elasticClientMock) IndexGet(indices ...string) *elastic.IndicesGetService {
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

type consumerMock struct {
	mock.Mock
}

func (c *consumerMock) Start(messageHandler func(message kafka.FTMessage)) {
	c.Called(messageHandler)
}

func (c *consumerMock) Close() error {
	args := c.Called()
	return args.Error(0)
}

func (c *consumerMock) ConnectivityCheck() error {
	args := c.Called()
	return args.Error(0)
}

func (c *consumerMock) MonitorCheck() error {
	args := c.Called()
	return args.Error(0)
}

func mockMessageHandler(esClient ESClient, mocks ...interface{}) (es.AccessConfig, *Handler) {
	uppLogger := logger.NewUPPLogger(config.AppName, config.AppDefaultLogLevel)

	accessConfig := es.AccessConfig{
		AwsCreds: &credentials.Credentials{},
		Endpoint: "endpoint",
	}

	var concordanceAPI *concordanceAPIMock
	var esService *esServiceMock
	var consumer *consumerMock

	for _, m := range mocks {
		switch m.(type) {
		case *concordanceAPIMock:
			concordanceAPI = m.(*concordanceAPIMock)
		case *esServiceMock:
			esService = m.(*esServiceMock)
		case *consumerMock:
			consumer = m.(*consumerMock)
		}
	}

	mapperHandler := mockMapperHandler(concordanceAPI, uppLogger)

	var handler *Handler

	opaAgent := mockOpaAgent{
		returnResult: &policy.ContentPolicyResult{},
	}

	if esService == nil {
		handler = NewMessageHandler(es.NewService("index"), mapperHandler, http.DefaultClient, consumer, esClient, uppLogger, opaAgent)
	} else {
		handler = NewMessageHandler(esService, mapperHandler, http.DefaultClient, consumer, esClient, uppLogger, opaAgent)
	}

	return accessConfig, handler
}

func mockMapperHandler(concordanceAPIMock *concordanceAPIMock, log *logger.UPPLogger) *mapper.Handler {
	appConfig := initAppConfig()
	mapperHandler := mapper.NewMapperHandler(concordanceAPIMock, "http://api.ft.com", appConfig, log)
	return mapperHandler
}

func initAppConfig() config.AppConfig {
	appConfig, err := config.ParseConfig("../../../configs/app.yml")
	if err != nil {
		log.Fatal(err)
	}
	return appConfig
}

func TestStartClient(t *testing.T) {
	expect := assert.New(t)

	consumer := &consumerMock{}
	consumer.On("Start", mock.AnythingOfType("func(kafka.FTMessage)")).Return()
	consumer.On("Close").Return(nil)

	accessConfig, handler := mockMessageHandler(defaultESClient, consumer)

	handler.Start("http://api.ft.com/", accessConfig)
	defer handler.Stop()

	time.Sleep(100 * time.Millisecond)

	expect.NotNil(handler.esService, "Elastic Service should be initialized")
	expect.Equal("index", handler.esService.(*es.ElasticsearchService).IndexName, "Wrong index")
	expect.NotNil(handler.esService.(*es.ElasticsearchService).GetClient(), "Elastic client should be initialized")
}
func TestStartClientError(t *testing.T) {
	expect := assert.New(t)

	consumer := &consumerMock{}
	consumer.On("Start", mock.AnythingOfType("func(kafka.FTMessage)")).Return()
	consumer.On("ConnectivityCheck").Return(fmt.Errorf("queue error"))
	consumer.On("MonitorCheck").Return(fmt.Errorf("monitor error"))
	consumer.On("Close").Return(nil)

	accessConfig, handler := mockMessageHandler(errorESClient, consumer)

	handler.Start("http://api.ft.com/", accessConfig)
	defer handler.Stop()

	time.Sleep(100 * time.Millisecond)

	expect.NotNil(handler.esService, "Elastic Service should be initialized")
	expect.Equal("index", handler.esService.(*es.ElasticsearchService).IndexName, "Wrong index")
	expect.Nil(handler.esService.(*es.ElasticsearchService).ElasticClient, "Elastic client should not be initialized")
	expect.EqualError(handler.consumer.ConnectivityCheck(), "queue error")
	expect.EqualError(handler.consumer.MonitorCheck(), "monitor error")
}

func TestHandleWriteMessage(t *testing.T) {
	expect := assert.New(t)

	inputJSON := testdata.ReadTestResource("exampleEnrichedContentModel.json")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResponse{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(kafka.FTMessage{Body: string(inputJSON)})

	expect.Equal(1, len(serviceMock.Calls))

	data := serviceMock.Calls[0].Arguments.Get(1)
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

	inputJSON := testdata.ReadTestResource("exampleEnrichedContentModelWithBodyXML.json")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResponse{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(kafka.FTMessage{Body: string(inputJSON)})

	expect.Equal(1, len(serviceMock.Calls))

	data := serviceMock.Calls[0].Arguments.Get(1)
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
	serviceMock.On("WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResponse{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(kafka.FTMessage{Body: input})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageBlogWithHeader(t *testing.T) {
	input := modifyTestInputAuthority("invalid")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResponse{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{"Origin-System-Id": "wordpress", "Content-Type": "application/json"}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageVideo(t *testing.T) {
	input := modifyTestInputAuthority("NEXT-VIDEO-EDITOR")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResponse{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{"Content-Type": "application/json"}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageAudio(t *testing.T) {
	input := modifyTestInputAuthority("NEXT-VIDEO-EDITOR")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResponse{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{"Content-Type": "vnd.ft-upp-audio+json"}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageAudioWithoutHeader(t *testing.T) {

	inputJSON := testdata.ReadTestResource("exampleAudioModel.json")
	input := strings.Replace(string(inputJSON), "FTCOM-METHODE", "NEXT-VIDEO-EDITOR", 1)

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResponse{}, nil)

	concordanceAPIMock := new(concordanceAPIMock)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(kafka.FTMessage{
		Body:    input,
		Headers: map[string]string{},
	})

	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageArticleByHeaderType(t *testing.T) {
	input := modifyTestInputAuthority("invalid")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResponse{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{"Content-Type": "application/vnd.ft-upp-article"}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageUnknownType(t *testing.T) {
	inputJSON := testdata.ReadTestResource("exampleEnrichedContentModel.json")

	input := strings.Replace(string(inputJSON), `"Article"`, `"Content"`, 1)

	serviceMock := &esServiceMock{}

	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(kafka.FTMessage{Body: input})

	serviceMock.AssertNotCalled(t, "WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060")
	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageNoUUIDForMetadataPublish(t *testing.T) {
	inputJSON := testdata.ReadTestResource("testEnrichedContentModel3.json")

	serviceMock := &esServiceMock{}

	_, h := mockMessageHandler(defaultESClient, serviceMock)
	methodeOrigin := h.mapper.Config.ContentMetadataMap.Get("methode").Origin
	h.handleMessage(kafka.FTMessage{
		Body: string(inputJSON),
		Headers: map[string]string{
			originHeader: methodeOrigin,
		},
	})

	serviceMock.AssertNotCalled(t, "WriteData", "b17756fe-0f62-4cf1-9deb-ca7a2ff80172", mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", "b17756fe-0f62-4cf1-9deb-ca7a2ff80172")
	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageNoType(t *testing.T) {
	input := modifyTestInputAuthority("invalid")

	serviceMock := &esServiceMock{}

	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(kafka.FTMessage{Body: input})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything)
}

func TestHandleWriteMessageError(t *testing.T) {
	inputJSON := testdata.ReadTestResource("exampleEnrichedContentModel.json")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResponse{}, elastic.ErrTimeout)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(kafka.FTMessage{Body: string(inputJSON)})

	serviceMock.AssertExpectations(t)

	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleDeleteMessage(t *testing.T) {
	inputJSON := testdata.ReadTestResource("exampleEnrichedContentModel.json")
	input := strings.Replace(string(inputJSON), `"deleted": false`, `"deleted": true`, 1)

	serviceMock := &esServiceMock{}
	serviceMock.On("DeleteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060").Return(&elastic.DeleteResponse{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(kafka.FTMessage{Body: input})

	serviceMock.AssertExpectations(t)
}

func TestHandleDeleteMessageError(t *testing.T) {
	inputJSON := testdata.ReadTestResource("exampleEnrichedContentModel.json")
	input := strings.Replace(string(inputJSON), `"deleted": false`, `"deleted": true`, 1)

	serviceMock := &esServiceMock{}

	serviceMock.On("DeleteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060").Return(&elastic.DeleteResponse{}, elastic.ErrTimeout)

	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(kafka.FTMessage{Body: input})

	serviceMock.AssertExpectations(t)
}

func TestHandleMessageJsonError(t *testing.T) {
	serviceMock := &esServiceMock{}
	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(kafka.FTMessage{Body: "malformed json"})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything)
}

func TestHandleSyntheticMessage(t *testing.T) {
	serviceMock := &esServiceMock{}
	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(kafka.FTMessage{Headers: map[string]string{"X-Request-Id": "SYNTHETIC-REQ-MON_WuLjbRpCgh"}})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything)
}

func TestHandleFTPinkAnnotationsMessage(t *testing.T) {
	serviceMock := &esServiceMock{}
	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.handleMessage(kafka.FTMessage{Headers: map[string]string{"Origin-System-Id": config.FTPinkAnnotationsOrigin}, Body: "{}"})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything)
}

func TestHandleFTPinkAnnotationsMessageWithOldSparkContent(t *testing.T) {
	input := modifyTestInputAuthority("cct")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResponse{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{originHeader: config.FTPinkAnnotationsOrigin}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleFTPinkAnnotationsMessageWithSparkContent(t *testing.T) {
	input := modifyTestInputAuthority("spark")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResponse{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.handleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{originHeader: config.FTPinkAnnotationsOrigin}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func modifyTestInputAuthority(replacement string) string {
	inputJSON := testdata.ReadTestResource("exampleEnrichedContentModel.json")
	input := strings.Replace(string(inputJSON), "FTCOM-METHODE", replacement, 1)
	return input
}
