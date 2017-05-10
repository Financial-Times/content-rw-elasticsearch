package main

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
	"time"
)

func TestConvertToESContentModel(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		inputFile  string
		outputFile string
	}{
		{"testdata/exampleEnrichedContentModel.json", "testdata/exampleElasticModel.json"},
		{"testdata/testInput1.json", "testdata/testOutput1.json"},
		{"testdata/testInput2.json", "testdata/testOutput2.json"},
		{"testdata/testInput3.json", "testdata/testOutput3.json"},
	}

	for _, test := range tests {
		ecModel := enrichedContentModel{}
		inputJSON, err := ioutil.ReadFile(test.inputFile)
		assert.NoError(err, "Unexpected error")

		err = json.Unmarshal([]byte(inputJSON), &ecModel)
		assert.NoError(err, "Unexpected error")

		startTime := time.Now().UnixNano() / 1000000
		esModel := convertToESContentModel(ecModel, "article")

		endTime := time.Now().UnixNano() / 1000000

		indexDate, err := time.Parse("2006-01-02T15:04:05.999Z", *esModel.IndexDate)
		assert.NoError(err, "Unexpected error")
		indexTime := indexDate.UnixNano() / 1000000

		assert.True(indexTime >= startTime && indexTime <= endTime, "Index date %s not correct", *esModel.IndexDate)

		esModel.IndexDate = nil

		expectedJSON, err := ioutil.ReadFile(test.outputFile)
		assert.NoError(err, "Unexpected error")

		expectedESModel := esContentModel{}
		err = json.Unmarshal([]byte(expectedJSON), &expectedESModel)
		assert.NoError(err, "Unexpected error")

		assert.Equal(expectedESModel, esModel)
	}
}

func TestCmrID(t *testing.T) {
	assert := assert.New(t)
	assert.Equal("NzE0ZThkZGItNDAyMC00MDRjLTlkNzMtY2I5MzRmZDVhOWM2-T04=",
		getCmrID("ON", []string{"YzcxMTcyNGYtMzQyZC00ZmU2LTk0ZGYtYWI2Y2YxMDMwMTQy-QXV0aG9ycw==", "NzE0ZThkZGItNDAyMC00MDRjLTlkNzMtY2I5MzRmZDVhOWM2-T04="}), "Wrong CMR ID")
}
