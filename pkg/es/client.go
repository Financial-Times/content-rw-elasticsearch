package es

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSigner "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/olivere/elastic/v7"
)

type Client interface {
	ClusterHealth() *elastic.ClusterHealthService
	Index() *elastic.IndexService
	Get() *elastic.GetService
	Delete() *elastic.DeleteService
	IndexGet(indices ...string) *elastic.IndicesGetService
}

type AccessConfig struct {
	AwsCreds *credentials.Credentials
	Endpoint string
	Region   string
}

type AWSSigningTransport struct {
	HTTPClient  *http.Client
	Credentials *credentials.Credentials
	Region      string
}

// RoundTrip implementation
func (a AWSSigningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	signer := awsSigner.NewSigner(a.Credentials)
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body with error: %w", err)
		}
		body := strings.NewReader(string(b))
		defer req.Body.Close()
		_, err = signer.Sign(req, body, "es", a.Region, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to sign request: %w", err)
		}
	} else {
		_, err := signer.Sign(req, nil, "es", a.Region, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to sign request: %w", err)
		}
	}

	return a.HTTPClient.Do(req)
}

func NewClient(config AccessConfig, c *http.Client, log *logger.UPPLogger) (Client, error) {
	signingTransport := AWSSigningTransport{
		Credentials: config.AwsCreds,
		HTTPClient:  c,
		Region:      config.Region,
	}
	signingClient := &http.Client{Transport: http.RoundTripper(signingTransport)}

	return elastic.NewClient(
		elastic.SetURL(config.Endpoint),
		elastic.SetScheme("https"),
		elastic.SetHttpClient(signingClient),
		elastic.SetSniff(false), // needs to be disabled due to EAS behavior. Healthcheck still operates as normal.
		elastic.SetErrorLog(log),
	)
}
