package policy

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/opa-client-go"
)

const (
	testDecisionID      = "1e58b3bf-995c-473e-90e9-ab1f10af74ab"
	errMsgPublicationSV = "publication array contains the Sustainable Views UUID: 8e6c705e-1132-42a2-8db0-c295e29e8658 while the Pink FT UUID: 88fdde6c-2aa4-4f78-af02-9f680097cfd6 is not present"
)

func TestAgent_EvaluateContentPolicy(t *testing.T) {
	tests := []struct {
		name           string
		server         *httptest.Server
		paths          map[string]string
		query          map[string]interface{} // query field is informative and is not being used during tests evaluation
		expectedResult *ContentPolicyResult
		expectedError  error
	}{
		{
			name: "Evaluate a Skipping Policy Decision",
			server: createHTTPTestServer(
				t,
				fmt.Sprintf(`{"decision_id": %q, "result": {"skip": true, "reasons": [%q]}}`, testDecisionID, errMsgPublicationSV),
			),
			paths: map[string]string{
				FilterSVContent: "content_rw_elasticsearch/content_msg_evaluator",
			},
			query: map[string]interface{}{
				"editorialDesk": "/FT/Professional/Central Banking", // This is informative and is not being used during test evaluation
			},
			expectedResult: &ContentPolicyResult{
				Skip:    true,
				Reasons: []string{errMsgPublicationSV},
			},
			expectedError: nil,
		},
		{
			name: "Evaluate a Non-Skipping Policy Decision",
			server: createHTTPTestServer(
				t,
				fmt.Sprintf(`{"decision_id": %q, "result": {"skip": false}}`, testDecisionID),
			),
			paths: map[string]string{
				FilterSVContent: "content_rw_elasticsearch/content_msg_evaluator",
			},
			query: map[string]interface{}{
				"editorialDesk": "/FT/Money", // This is informative and is not being used during test evaluation
			},
			expectedResult: &ContentPolicyResult{
				Skip: false,
			},
			expectedError: nil,
		},
		{
			name: "Evaluate and Receive an Error.",
			server: createHTTPTestServer(
				t,
				``,
			),
			paths:          make(map[string]string),
			query:          make(map[string]interface{}),
			expectedResult: nil,
			expectedError:  ErrEvaluatePolicy,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(_ *testing.T) {
			defer test.server.Close()

			l := logger.NewUPPLogger("content-rw-elasticsearch", "INFO")
			c := opa.NewOpenPolicyAgentClient(test.server.URL, test.paths, opa.WithLogger(l))

			o := NewOpenPolicyAgent(c, l)

			result, err := o.EvaluateContentPolicy(test.query)

			if err != nil {
				if !errors.Is(err, test.expectedError) {
					t.Errorf(
						"Unexpected error received from call to EvaluateContentPolicy: %v",
						err,
					)
				}
			} else {
				assert.Equal(t, test.expectedResult, result)
			}
		})
	}
}

func createHTTPTestServer(t *testing.T, response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(response))
		if err != nil {
			t.Fatalf("could not write response from test http server: %v", err)
		}
	}))
}
