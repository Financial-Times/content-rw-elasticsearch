package main

import (
	awsauth "github.com/smartystreets/go-aws-auth"
	"gopkg.in/olivere/elastic.v2"
	"log"
	"net/http"
	"os"
)

type esClientI interface {
	ClusterHealth() *elastic.ClusterHealthService
	Index() *elastic.IndexService
	Get() *elastic.GetService
	Delete() *elastic.DeleteService
	IndexGet() *elastic.IndicesGetService
}

type esAccessConfig struct {
	accessKey  string
	secretKey  string
	esEndpoint string
}

type AWSSigningTransport struct {
	HTTPClient  *http.Client
	Credentials awsauth.Credentials
}

// RoundTrip implementation
func (a AWSSigningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return a.HTTPClient.Do(awsauth.Sign4(req, a.Credentials))
}

var newAmazonClient = func(config esAccessConfig) (esClientI, error) {

	signingTransport := AWSSigningTransport{
		Credentials: awsauth.Credentials{
			AccessKeyID:     config.accessKey,
			SecretAccessKey: config.secretKey,
		},
		HTTPClient: http.DefaultClient,
	}
	signingClient := &http.Client{Transport: http.RoundTripper(signingTransport)}

	return elastic.NewClient(
		elastic.SetURL(config.esEndpoint),
		elastic.SetScheme("https"),
		elastic.SetHttpClient(signingClient),
		elastic.SetSniff(false), //needs to be disabled due to EAS behavior. Healthcheck still operates as normal.
		elastic.SetInfoLog(log.New(os.Stderr, "", log.LstdFlags)),
		elastic.SetErrorLog(log.New(os.Stderr, "", log.LstdFlags)),
		elastic.SetTraceLog(log.New(os.Stderr, "", log.LstdFlags)),
	)
}
