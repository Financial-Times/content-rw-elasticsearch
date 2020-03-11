package concept

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockHttpClient struct {
	mock.Mock
}

func (c *mockHttpClient) Do(req *http.Request) (resp *http.Response, err error) {
	args := c.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

type mockResponseBody struct {
	mock.Mock
}

func (b *mockResponseBody) Read(p []byte) (n int, err error) {
	args := b.Called(p)
	return args.Int(0), args.Error(1)
}

func (b *mockResponseBody) Close() error {
	args := b.Called()
	return args.Error(0)
}

type mockConcordanceApiServer struct {
	mock.Mock
}

func (m *mockConcordanceApiServer) RequestConcordances(tid, acceptHeader string, ids []string) (status int, body []byte) {
	args := m.Called(tid, acceptHeader, ids)
	return args.Int(0), args.Get(1).([]byte)
}

func (m *mockConcordanceApiServer) GTG() int {
	args := m.Called()
	return args.Int(0)
}

func (m *mockConcordanceApiServer) startMockServer(t *testing.T) *httptest.Server {
	router := mux.NewRouter()
	router.HandleFunc("/concordances", func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		assert.Equal(t, "UPP content-rw-elasticsearch", ua)

		acceptHeader := r.Header.Get("Accept")
		tid := r.Header.Get("X-Request-Id")

		query, err := url.ParseQuery(r.URL.RawQuery)
		assert.NoError(t, err)
		respStatus, respBody := m.RequestConcordances(tid, acceptHeader, query["conceptId"])
		w.WriteHeader(respStatus)
		w.Write(respBody)
	}).Methods(http.MethodGet)

	router.HandleFunc("/__gtg", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(m.GTG())
	}).Methods(http.MethodGet)

	return httptest.NewServer(router)
}

func TestConcordanceApiService_GetConceptsSuccessfully(t *testing.T) {
	expect := assert.New(t)

	sampleID := ThingURIPrefix + uuid.NewRandom().String()

	sampleResponse := ConcordancesResponse{
		Concordances: []Concordance{
			{
				Concept: Concept{
					ID:     sampleID,
					APIURL: sampleID,
				},
				Identifier: Identifier{
					Authority:       tmeAuthority,
					IdentifierValue: "TME-ID",
				},
			},
		},
	}
	body, err := json.Marshal(&sampleResponse)
	expect.NoError(err)

	expected := map[string]Model{
		sampleID: {TmeIDs: []string{"TME-ID"}},
	}

	mockServer := new(mockConcordanceApiServer)
	mockServer.On("RequestConcordances", "tid_test", "application/json", []string{sampleID}).Return(http.StatusOK, body)
	server := mockServer.startMockServer(t)

	concordanceAPIService := NewConcordanceAPIService(server.URL, http.DefaultClient)

	concepts, err := concordanceAPIService.GetConcepts("tid_test", []string{sampleID})

	expect.NoError(err)
	expect.Equal(expected, concepts)
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestConcordanceApiService_GetConceptsServiceUnavailable(t *testing.T) {
	expect := assert.New(t)

	sampleID := ThingURIPrefix + uuid.NewRandom().String()

	mockServer := new(mockConcordanceApiServer)
	mockServer.On("RequestConcordances", "tid_test", "application/json", []string{sampleID}).Return(http.StatusServiceUnavailable, []byte{})
	server := mockServer.startMockServer(t)

	concordanceAPIService := NewConcordanceAPIService(server.URL, http.DefaultClient)

	concepts, err := concordanceAPIService.GetConcepts("tid_test", []string{sampleID})

	expect.Error(err)
	expect.Equal("calling Concordance API returned HTTP status 503", err.Error())
	expect.Nil(concepts)
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestConcordanceApiService_GetConceptsErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)

	sampleID := ThingURIPrefix + uuid.NewRandom().String()

	concordanceAPIService := NewConcordanceAPIService(":/", http.DefaultClient)

	concepts, err := concordanceAPIService.GetConcepts("tid_test", []string{sampleID})

	expect.Error(err)
	expect.Nil(concepts)
}

func TestConcordanceApiService_GetConceptsErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("http client err"))

	sampleID := ThingURIPrefix + uuid.NewRandom().String()

	concordanceAPIService := NewConcordanceAPIService("http://test-url", mockClient)

	concepts, err := concordanceAPIService.GetConcepts("tid_test", []string{sampleID})

	expect.Error(err)
	expect.Equal("http client err", err.Error())
	expect.Nil(concepts)
	mock.AssertExpectationsForObjects(t, mockClient)
}

func TestConcordanceApiService_GetConceptsErrorOnResponseBodyRead(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockBody := new(mockResponseBody)

	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{Body: mockBody, StatusCode: http.StatusOK}, nil)
	mockBody.On("Read", mock.AnythingOfType("[]uint8")).Return(0, errors.New("read err"))
	mockBody.On("Close").Return(nil)

	sampleID := ThingURIPrefix + uuid.NewRandom().String()

	concordanceAPIService := NewConcordanceAPIService("http://test-url", mockClient)

	concepts, err := concordanceAPIService.GetConcepts("tid_test", []string{sampleID})

	expect.Error(err)
	expect.Equal("read err", err.Error())
	expect.Nil(concepts)
	mock.AssertExpectationsForObjects(t, mockClient)
	mock.AssertExpectationsForObjects(t, mockBody)
}

func TestConcordanceApiService_GetConceptsErrorOnInvalidJSONResponse(t *testing.T) {
	expect := assert.New(t)

	sampleID := ThingURIPrefix + uuid.NewRandom().String()

	mockServer := new(mockConcordanceApiServer)
	mockServer.On("RequestConcordances", "tid_test", "application/json", []string{sampleID}).Return(http.StatusOK, []byte("{invalid JSON}"))
	server := mockServer.startMockServer(t)

	concordanceAPIService := NewConcordanceAPIService(server.URL, http.DefaultClient)

	concepts, err := concordanceAPIService.GetConcepts("tid_test", []string{sampleID})

	expect.Error(err)
	expect.Equal("invalid character 'i' looking for beginning of object key string", err.Error())
	expect.Nil(concepts)
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestConcordanceApiService_CheckHealthSuccessfully(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockConcordanceApiServer)
	mockServer.On("GTG").Return(http.StatusOK)
	server := mockServer.startMockServer(t)

	concordanceAPIService := NewConcordanceAPIService(server.URL, http.DefaultClient)

	check, err := concordanceAPIService.HealthCheck()
	expect.NoError(err)
	expect.Equal("Concordance API is healthy", check)
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestConcordanceApiService_CheckHealthUnhealthy(t *testing.T) {
	expect := assert.New(t)
	mockServer := new(mockConcordanceApiServer)
	mockServer.On("GTG").Return(http.StatusServiceUnavailable)
	server := mockServer.startMockServer(t)

	concordanceAPIService := NewConcordanceAPIService(server.URL, http.DefaultClient)

	check, err := concordanceAPIService.HealthCheck()
	expect.Error(err)
	expect.Empty(check)
	expect.Equal("Health check returned a non-200 HTTP status: 503", err.Error())
	mock.AssertExpectationsForObjects(t, mockServer)
}

func TestConcordanceApiService_CheckHealthErrorOnNewRequest(t *testing.T) {
	expect := assert.New(t)

	concordanceAPIService := NewConcordanceAPIService(":/", http.DefaultClient)

	check, err := concordanceAPIService.HealthCheck()
	expect.Error(err)
	expect.Empty(check)
}

func TestConcordanceApiService_CheckHealthErrorOnRequestDo(t *testing.T) {
	expect := assert.New(t)
	mockClient := new(mockHttpClient)
	mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{}, errors.New("http client err"))

	concordanceAPIService := NewConcordanceAPIService("http://test-url", mockClient)

	check, err := concordanceAPIService.HealthCheck()
	expect.Error(err)
	expect.Empty(check)
	expect.Equal("http client err", err.Error())
}
