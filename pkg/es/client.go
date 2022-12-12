package es

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
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
	AWSConfig aws.Config
	Endpoint  string
	Region    string
}

type AWSSigningTransport struct {
	HTTPClient *http.Client
	AWSConfig  aws.Config
	Region     string
}

func (a AWSSigningTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	// If the region is local that means that we probably want to run dredd tests, so our requests won't get signed!
	if a.Region == "local" {
		return a.HTTPClient.Do(req)
	}

	defer func() {
		log := logger.NewUPPLogger("roundtripper", "INFO")
		if err != nil {
			log.WithError(err).Error("ERROR")
		} else {
			log.Info("Is it good?")
		}
	}()

	credentials, err := a.AWSConfig.Credentials.Retrieve(req.Context())
	if err != nil {
		return nil, err
	}

	internalReq := req.Clone(req.Context())

	bodyReader, err := req.GetBody()
	if err != nil {
		return nil, fmt.Errorf("reading request body: %w", err)
	}

	hash := sha256.New()

	if _, err := io.Copy(hash, bodyReader); err != nil {
		return nil, fmt.Errorf("copying request body: %w", err)
	}

	if err := signer.
		NewSigner().
		SignHTTP(context.Background(), credentials, internalReq, hex.EncodeToString(hash.Sum(nil)), "es", a.Region, time.Now()); err != nil {
		return nil, fmt.Errorf("signing request: %w", err)
	}

	return a.HTTPClient.Do(internalReq)
}

func NewClient(config AccessConfig, client *http.Client, log *logger.UPPLogger) (Client, error) {
	signingTransport := AWSSigningTransport{
		AWSConfig:  config.AWSConfig,
		HTTPClient: client,
		Region:     config.Region,
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
