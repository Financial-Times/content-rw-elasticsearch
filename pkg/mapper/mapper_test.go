package mapper

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Financial-Times/content-rw-elasticsearch/pkg/config"
	"github.com/Financial-Times/content-rw-elasticsearch/pkg/schema"
	tst "github.com/Financial-Times/content-rw-elasticsearch/test"
	"github.com/Financial-Times/go-logger/v2"

	"github.com/Financial-Times/content-rw-elasticsearch/pkg/concept"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type concordanceAPIMock struct {
	mock.Mock
}

func (m *concordanceAPIMock) GetConcepts(tid string, ids []string) (map[string]concept.Model, error) {
	args := m.Called(tid, ids)
	return args.Get(0).(map[string]concept.Model), args.Error(1)
}

func TestConvertToESContentModel(t *testing.T) {
	expect := assert.New(t)

	tests := []struct {
		contentType               string
		inputFileEnrichedModel    string
		inputFileConcordanceModel string
		outputFile                string
		tid                       string
	}{
		{config.ArticleType, "exampleEnrichedContentModel.json", "exampleConcordanceResponse.json", "exampleElasticModel.json", "tid_1"},
		{config.ArticleType, "testEnrichedContentModel1.json", "testConcordanceResponse1.json", "testElasticModel1.json", "tid_2"},
		{config.ArticleType, "testEnrichedContentModel2.json", "", "testElasticModel2.json", "tid_3"},
		{config.VideoType, "testEnrichedContentModel4.json", "", "testElasticModel4.json", "tid_video"},
	}

	log := logger.NewUPPLogger(config.AppName, config.AppDefaultLogLevel)
	appConfig, err := config.ParseConfig("app.yml")
	if err != nil {
		log.Fatal(err)
	}
	concordanceAPIMock := new(concordanceAPIMock)
	mapperHandler := NewMapperHandler(concordanceAPIMock, "http://api.ft.com", appConfig, log)

	for _, test := range tests {
		if test.inputFileConcordanceModel != "" {
			inputConcordanceJSON := tst.ReadTestResource(test.inputFileConcordanceModel)

			var concResp concept.ConcordancesResponse
			err = json.Unmarshal(inputConcordanceJSON, &concResp)
			require.NoError(t, err, "Unexpected error")

			concordanceAPIMock.On("GetConcepts", test.tid, mock.AnythingOfType("[]string")).Return(concept.TransformToConceptModel(concResp), nil)
		}
		ecModel := schema.EnrichedContent{}
		inputJSON := tst.ReadTestResource(test.inputFileEnrichedModel)

		err = json.Unmarshal(inputJSON, &ecModel)
		require.NoError(t, err, "Unexpected error")

		milliseconds := int64(time.Millisecond)
		startTime := time.Now().UnixNano() / milliseconds
		esModel := mapperHandler.ToIndexModel(ecModel, test.contentType, test.tid)

		endTime := time.Now().UnixNano() / milliseconds

		indexDate, err := time.Parse("2006-01-02T15:04:05.999Z", *esModel.IndexDate)
		expect.NoError(err, "Unexpected error")
		indexTime := indexDate.UnixNano() / 1000000

		expect.True(indexTime >= startTime && indexTime <= endTime, "Index date %s not correct", *esModel.IndexDate)

		esModel.IndexDate = nil

		expectedJSON := tst.ReadTestResource(test.outputFile)
		expectedESModel := schema.IndexModel{}
		err = json.Unmarshal(expectedJSON, &expectedESModel)
		expect.NoError(err, "Unexpected error")

		// the publishRef field is actually overwritten with the x-request-header received from the message, instead of the one read from doc-store
		expectedESModel.PublishReference = test.tid

		expect.Equal(expectedESModel, esModel, "ES model not matching with the one from %v", test.outputFile)

		mock.AssertExpectationsForObjects(t, concordanceAPIMock)
	}
}

func TestCmrID(t *testing.T) {
	expect := assert.New(t)
	cmrID, found := getCmrID("ON", []string{"YzcxMTcyNGYtMzQyZC00ZmU2LTk0ZGYtYWI2Y2YxMDMwMTQy-QXV0aG9ycw==", "NzE0ZThkZGItNDAyMC00MDRjLTlkNzMtY2I5MzRmZDVhOWM2-T04="})
	expect.True(found, "CMR ID is not composed from the expected taxonomy")
	expect.Equal("NzE0ZThkZGItNDAyMC00MDRjLTlkNzMtY2I5MzRmZDVhOWM2-T04=", cmrID, "Wrong CMR ID")
}
