package kafka

import (
	"bytes"
	"regexp"
	"strings"
)

type FTMessage struct {
	Headers map[string]string
	Body    string
}

func NewFTMessage(headers map[string]string, body string) FTMessage {
	return FTMessage{
		Headers: headers,
		Body:    body,
	}
}

func (m *FTMessage) Build() string {
	var buffer bytes.Buffer
	buffer.WriteString("FTMSG/1.0\r\n")

	for k, v := range m.Headers {
		buffer.WriteString(k)
		buffer.WriteString(": ")
		buffer.WriteString(v)
		buffer.WriteString("\r\n")
	}
	buffer.WriteString("\r\n")
	buffer.WriteString(m.Body)

	return buffer.String()
}

func rawToFTMessage(msg []byte) FTMessage {
	ftMsg := FTMessage{}
	raw := string(msg)

	doubleNewLineStartIndex := getHeaderSectionEndingIndex(string(raw[:]))
	ftMsg.Headers = parseHeaders(string(raw[:doubleNewLineStartIndex]))
	ftMsg.Body = strings.TrimSpace(string(raw[doubleNewLineStartIndex:]))
	return ftMsg
}

var re = regexp.MustCompile("[\\w-]*:[\\w\\-:/.+;= ]*")
var kre = regexp.MustCompile("[\\w-]*:")
var vre = regexp.MustCompile(":[\\w-:/.+;= ]*")

func getHeaderSectionEndingIndex(msg string) int {
	//FT msg format uses CRLF for line endings
	i := strings.Index(msg, "\r\n\r\n")
	if i != -1 {
		return i
	}
	//fallback to UNIX line endings
	i = strings.Index(msg, "\n\n")
	if i != -1 {
		return i
	}
	return len(msg)
}

func parseHeaders(msg string) map[string]string {
	headerLines := re.FindAllString(msg, -1)

	headers := make(map[string]string)
	for _, line := range headerLines {
		key, value := parseHeader(line)
		headers[key] = value
	}
	return headers
}
func parseHeader(header string) (string, string) {
	key := kre.FindString(header)
	value := vre.FindString(header)
	return key[:len(key)-1], strings.TrimSpace(value[1:])
}
