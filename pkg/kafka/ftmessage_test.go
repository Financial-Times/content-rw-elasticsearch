package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testFTMessage               = "FTMSG/1.0\r\ntest: test2\r\n\r\nTest Message"
	testComplexFTMessage        = "FTMSG/1.0\r\nContent-Type: application/vnd.ft-upp-article+json; version=1.0; charset=utf-8\r\n\r\nTest Message"
	testFTMessageHeaders        = map[string]string{"test": "test2"}
	testFTMessageBody           = "Test Message"
	testComplexFTMessageHeaders = map[string]string{"Content-Type": "application/vnd.ft-upp-article+json; version=1.0; charset=utf-8"}
)

func Test_FTMessage_Build(t *testing.T) {
	ft := NewFTMessage(testFTMessageHeaders, testFTMessageBody)

	ftm := ft.Build()
	assert.EqualValues(t, testFTMessage, ftm)
}

func TestFTMessage_Parse(t *testing.T) {
	payload := []byte(testFTMessage)
	ftmsg := rawToFTMessage(payload)

	assert.EqualValues(t, ftmsg.Headers, testFTMessageHeaders)
	assert.EqualValues(t, ftmsg.Body, testFTMessageBody)
}

func TestFTMessage_Parse_NoBody(t *testing.T) {
	payload := []byte("FTMSG/1.0\r\ntest: test2")
	ftmsg := rawToFTMessage(payload)

	assert.EqualValues(t, ftmsg.Headers, testFTMessageHeaders)
	assert.EqualValues(t, ftmsg.Body, "")
}

func TestFTMessage_Parse_CRLF(t *testing.T) {
	payload := []byte("FTMSG/1.0\r\ntest: test2\r\n\r\nTest Message")
	ftmsg := rawToFTMessage(payload)

	assert.EqualValues(t, ftmsg.Headers, testFTMessageHeaders)
	assert.EqualValues(t, ftmsg.Body, testFTMessageBody)
}
func TestFTMessageWithComplexHeaders_Parse_CRLF(t *testing.T) {
	payload := []byte(testComplexFTMessage)
	ftmsg := rawToFTMessage(payload)

	assert.EqualValues(t, ftmsg.Headers, testComplexFTMessageHeaders)
	assert.EqualValues(t, ftmsg.Body, testFTMessageBody)
}
