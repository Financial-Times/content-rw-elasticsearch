package message

import (
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/schema"
	tst "github.com/Financial-Times/content-rw-elasticsearch/v2/test"
	"github.com/Financial-Times/kafka-client-go/v2"

	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/olivere/elastic.v2"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/upp-go-sdk/pkg/api"
	"github.com/Financial-Times/upp-go-sdk/pkg/internalcontent"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/concept"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/config"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/es"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/mapper"
)

type esServiceMock struct {
	mock.Mock
	client es.Client
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

func (s *esServiceMock) GetClient() es.Client {
	return s.client
}

func (s *esServiceMock) SetClient(client es.Client) {
	s.client = client
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

	internalAPIConfig := api.NewConfig("internalContentAPIURL", "apiBasicAuthUsername", "apiBasicAuthPassword")
	internalContentAPIClient := api.NewClient(*internalAPIConfig, http.DefaultClient)
	internalContentClient := internalcontent.NewContentClient(internalContentAPIClient, internalcontent.URLInternalContent)

	mapperHandler := mockMapperHandler(concordanceAPI, uppLogger, internalContentClient)
	mapperHandler.BaseAPIURL = "http://api.ft.com/"

	client, _ := esClient(accessConfig, http.DefaultClient, uppLogger)
	esService.SetClient(client)

	handler := NewMessageHandler(esService, mapperHandler, http.DefaultClient, uppLogger)
	if mocks == nil {
		elasticService := es.NewService("index")
		elasticService.SetClient(client)
		handler = NewMessageHandler(elasticService, mapperHandler, http.DefaultClient, uppLogger)
	}
	return accessConfig, handler
}

func mockMapperHandler(concordanceAPIMock *concordanceAPIMock, log *logger.UPPLogger, internalClient *internalcontent.ContentClient) *mapper.Handler {
	appConfig := initAppConfig()
	mapperHandler := mapper.NewMapperHandler(concordanceAPIMock, "http://api.ft.com", appConfig, log, internalClient)
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

	_, handler := mockMessageHandler(defaultESClient)

	time.Sleep(100 * time.Millisecond)

	expect.NotNil(handler.esService, "Elastic Service should be initialized")
	expect.Equal("index", handler.esService.(*es.ElasticsearchService).IndexName, "Wrong index")
	expect.NotNil(handler.esService.(*es.ElasticsearchService).GetClient(), "Elastic client should be initialized")
}
func TestStartClientError(t *testing.T) {
	expect := assert.New(t)

	_, handler := mockMessageHandler(errorESClient)

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
	handler.HandleMessage(kafka.FTMessage{Body: string(inputJSON)})

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
	handler.HandleMessage(kafka.FTMessage{Body: string(inputJSON)})

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
	handler.HandleMessage(kafka.FTMessage{Body: input})

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
	handler.HandleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{"Origin-System-Id": "wordpress", "Content-Type": "application/json"}})

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
	handler.HandleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{"Content-Type": "application/json"}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageAudio(t *testing.T) {
	input := modifyTestInputAuthority("NEXT-VIDEO-EDITOR")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTPodcasts", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.HandleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{"Content-Type": "vnd.ft-upp-audio+json"}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageAudioWithoutHeader(t *testing.T) {

	inputJSON := tst.ReadTestResource("exampleAudioModel.json")
	input := strings.Replace(string(inputJSON), "FTCOM-METHODE", "NEXT-VIDEO-EDITOR", 1)

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTPodcasts", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)

	concordanceAPIMock := new(concordanceAPIMock)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.HandleMessage(kafka.FTMessage{
		Body:    input,
		Headers: map[string]string{},
	})

	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageArticleByHeaderType(t *testing.T) {
	input := modifyTestInputAuthority("invalid")

	serviceMock := &esServiceMock{}
	serviceMock.On("WriteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything).Return(&elastic.IndexResult{}, nil)
	concordanceAPIMock := new(concordanceAPIMock)
	concordanceAPIMock.On("GetConcepts", mock.AnythingOfType("string"), mock.AnythingOfType("[]string")).Return(map[string]concept.Model{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock, concordanceAPIMock)
	handler.HandleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{"Content-Type": "application/vnd.ft-upp-article"}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleWriteMessageUnknownType(t *testing.T) {
	inputJSON := tst.ReadTestResource("exampleEnrichedContentModel.json")

	input := strings.Replace(string(inputJSON), `"Article"`, `"Content"`, 1)

	serviceMock := &esServiceMock{}

	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.HandleMessage(kafka.FTMessage{Body: input})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, "aae9611e-f66c-4fe4-a6c6-2e2bdea69060", mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, "aae9611e-f66c-4fe4-a6c6-2e2bdea69060")
	serviceMock.AssertExpectations(t)
}

func TestHandleWriteMessageNoUUIDForMetadataPublish(t *testing.T) {
	inputJSON := tst.ReadTestResource("testEnrichedContentModel3.json")

	serviceMock := &esServiceMock{}

	_, h := mockMessageHandler(defaultESClient, serviceMock)
	methodeOrigin := h.Mapper.Config.ContentMetadataMap.Get("methode").Origin
	h.HandleMessage(kafka.FTMessage{
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
	handler.HandleMessage(kafka.FTMessage{Body: input})

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
	handler.HandleMessage(kafka.FTMessage{Body: string(inputJSON)})

	serviceMock.AssertExpectations(t)

	concordanceAPIMock.AssertExpectations(t)
}

func TestHandleDeleteMessage(t *testing.T) {
	inputJSON := tst.ReadTestResource("exampleEnrichedContentModel.json")
	input := strings.Replace(string(inputJSON), `"deleted": false`, `"deleted": true`, 1)

	serviceMock := &esServiceMock{}
	serviceMock.On("DeleteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060").Return(&elastic.DeleteResult{}, nil)

	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.HandleMessage(kafka.FTMessage{Body: input})

	serviceMock.AssertExpectations(t)
}

func TestHandleDeleteMessageError(t *testing.T) {
	inputJSON := tst.ReadTestResource("exampleEnrichedContentModel.json")
	input := strings.Replace(string(inputJSON), `"deleted": false`, `"deleted": true`, 1)

	serviceMock := &esServiceMock{}

	serviceMock.On("DeleteData", "FTCom", "aae9611e-f66c-4fe4-a6c6-2e2bdea69060").Return(&elastic.DeleteResult{}, elastic.ErrTimeout)

	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.HandleMessage(kafka.FTMessage{Body: input})

	serviceMock.AssertExpectations(t)
}

func TestHandleMessageJsonError(t *testing.T) {
	serviceMock := &esServiceMock{}
	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.HandleMessage(kafka.FTMessage{Body: "malformed json"})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, mock.Anything)
}

func TestHandleSyntheticMessage(t *testing.T) {
	serviceMock := &esServiceMock{}
	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.HandleMessage(kafka.FTMessage{Headers: map[string]string{"X-Request-Id": "SYNTHETIC-REQ-MON_WuLjbRpCgh"}})

	serviceMock.AssertNotCalled(t, "WriteData", mock.Anything, mock.Anything, mock.Anything)
	serviceMock.AssertNotCalled(t, "DeleteData", mock.Anything, mock.Anything)
}

func TestHandlePACMessage(t *testing.T) {
	serviceMock := &esServiceMock{}
	_, handler := mockMessageHandler(defaultESClient, serviceMock)
	handler.HandleMessage(kafka.FTMessage{Headers: map[string]string{"Origin-System-Id": config.PACOrigin}, Body: "{}"})

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
	handler.HandleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{originHeader: config.PACOrigin}})

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
	handler.HandleMessage(kafka.FTMessage{Body: input, Headers: map[string]string{originHeader: config.PACOrigin}})

	serviceMock.AssertExpectations(t)
	concordanceAPIMock.AssertExpectations(t)
}

func modifyTestInputAuthority(replacement string) string {
	inputJSON := tst.ReadTestResource("exampleEnrichedContentModel.json")
	input := strings.Replace(string(inputJSON), "FTCOM-METHODE", replacement, 1)
	return input
}
