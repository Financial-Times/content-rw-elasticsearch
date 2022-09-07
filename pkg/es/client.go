package es

import (
	"net/http"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/olivere/elastic/v7"
	awsauth "github.com/smartystreets/go-aws-auth"
)

type Client interface {
	ClusterHealth() *elastic.ClusterHealthService
	Index() *elastic.IndexService
	Get() *elastic.GetService
	Delete() *elastic.DeleteService
	IndexGet(indices ...string) *elastic.IndicesGetService
}

type AccessConfig struct {
	AccessKey string
	SecretKey string
	Endpoint  string
}

type AWSSigningTransport struct {
	HTTPClient  *http.Client
	Credentials awsauth.Credentials
}

// RoundTrip implementation
func (a AWSSigningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return a.HTTPClient.Do(awsauth.Sign4(req, a.Credentials))
}

func NewClient(config AccessConfig, c *http.Client, log *logger.UPPLogger) (Client, error) {
	signingTransport := AWSSigningTransport{
		Credentials: awsauth.Credentials{
			AccessKeyID:     config.AccessKey,
			SecretAccessKey: config.SecretKey,
		},
		HTTPClient: c,
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
