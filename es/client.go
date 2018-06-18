package es

import (
	"net/http"

	"github.com/Financial-Times/go-logger"
	"github.com/smartystreets/go-aws-auth"
	"gopkg.in/olivere/elastic.v2"
)

type ClientI interface {
	ClusterHealth() *elastic.ClusterHealthService
	Bulk() *elastic.BulkService
	Index() *elastic.IndexService
	Get() *elastic.GetService
	Delete() *elastic.DeleteService
	IndexGet() *elastic.IndicesGetService
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

func NewClient(config AccessConfig, c *http.Client) (ClientI, error) {
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
		elastic.SetSniff(false), //needs to be disabled due to EAS behavior. Healthcheck still operates as normal.
		elastic.SetErrorLog(logger.Logger()),
	)
}
