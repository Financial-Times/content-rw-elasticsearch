package service

import (
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/content"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/service/concept"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConvertToESContentModel(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		contentType               string
		inputFileEnrichedModel    string
		inputFileConcordanceModel string
		outputFile                string
		tid                       string
	}{
		{"article", "testdata/exampleEnrichedContentModel.json", "testdata/exampleConcordanceResponse.json", "testdata/exampleElasticModel.json", "tid_1"},
		{"article", "testdata/testEnrichedContentModel1.json", "testdata/testConcordanceResponse1.json", "testdata/testElasticModel1.json", "tid_2"},
		{"article", "testdata/testEnrichedContentModel2.json", "", "testdata/testElasticModel2.json", "tid_3"},
		{"video", "testdata/testEnrichedContentModel4.json", "", "testdata/testElasticModel4.json", "tid_video"},
	}
	concordanceApiMock := new(concordanceApiMock)
	handler := &MessageHandler{ConceptGetter: concordanceApiMock, baseApiUrl: "http://api.ft.com"}

	for _, test := range tests {
		if test.inputFileConcordanceModel != "" {
			inputConcordanceJSON, err := ioutil.ReadFile(test.inputFileConcordanceModel)
			require.NoError(t, err, "Unexpected error")

			var concResp concept.ConcordancesResponse
			err = json.Unmarshal([]byte(inputConcordanceJSON), &concResp)
			require.NoError(t, err, "Unexpected error")

			concordanceApiMock.On("GetConcepts", test.tid, mock.AnythingOfType("[]string")).Return(concept.TransformToConceptModel(concResp), nil)
		}
		ecModel := content.EnrichedContent{}
		inputJSON, err := ioutil.ReadFile(test.inputFileEnrichedModel)
		require.NoError(t, err, "Unexpected error")

		err = json.Unmarshal([]byte(inputJSON), &ecModel)
		require.NoError(t, err, "Unexpected error")

		startTime := time.Now().UnixNano() / 1000000
		esModel := handler.ToIndexModel(ecModel, test.contentType, test.tid)

		endTime := time.Now().UnixNano() / 1000000

		indexDate, err := time.Parse("2006-01-02T15:04:05.999Z", *esModel.IndexDate)
		assert.NoError(err, "Unexpected error")
		indexTime := indexDate.UnixNano() / 1000000

		assert.True(indexTime >= startTime && indexTime <= endTime, "Index date %s not correct", *esModel.IndexDate)

		esModel.IndexDate = nil

		expectedJSON, err := ioutil.ReadFile(test.outputFile)
		assert.NoError(err, "Unexpected error")

		expectedESModel := content.IndexModel{}
		err = json.Unmarshal([]byte(expectedJSON), &expectedESModel)
		assert.NoError(err, "Unexpected error")

		//the publishRef field is actually overwritten with the x-request-header received from the message, instead of the one read from doc-store
		expectedESModel.PublishReference = test.tid
		assert.Equal(expectedESModel, esModel, "ES model not matching with the one from %v", test.outputFile)

		mock.AssertExpectationsForObjects(t, concordanceApiMock)
	}
}

func TestCmrID(t *testing.T) {
	assert := assert.New(t)
	cmrID, found := getCmrID("ON", []string{"YzcxMTcyNGYtMzQyZC00ZmU2LTk0ZGYtYWI2Y2YxMDMwMTQy-QXV0aG9ycw==", "NzE0ZThkZGItNDAyMC00MDRjLTlkNzMtY2I5MzRmZDVhOWM2-T04="})
	assert.True(found, "CMR ID is not composed from the expected taxonomy")
	assert.Equal("NzE0ZThkZGItNDAyMC00MDRjLTlkNzMtY2I5MzRmZDVhOWM2-T04=", cmrID, "Wrong CMR ID")
}
