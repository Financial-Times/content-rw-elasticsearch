package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Financial-Times/content-rw-elasticsearch/es"
	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/Financial-Times/service-status-go/gtg"
	"github.com/Financial-Times/content-rw-elasticsearch/concept"
)

type healthService struct {
	esHealthService  es.HealthServiceI
	concordanceApi   *concept.ConcordanceApiService
	consumerInstance consumer.MessageConsumer
	httpClient       *http.Client
	checks           []health.Check
	appSystemCode    string
}

func newHealthService(config *consumer.QueueConfig, esHealthService es.HealthServiceI, client *http.Client, concordanceApi *concept.ConcordanceApiService, appSystemCode string) *healthService {
	consumerInstance := consumer.NewConsumer(*config, func(m consumer.Message) {}, client)
	service := &healthService{
		esHealthService:  esHealthService,
		concordanceApi:   concordanceApi,
		consumerInstance: consumerInstance,
		httpClient:       client,
		appSystemCode:    appSystemCode,
	}
	service.checks = []health.Check{
		service.clusterIsHealthyCheck(),
		service.connectivityHealthyCheck(),
		service.schemaHealthyCheck(),
		service.checkKafkaProxyConnectivity(),
		service.checkConcordanceAPI(),
	}
	return service
}

func (service *healthService) clusterIsHealthyCheck() health.Check {
	return health.Check{
		ID:               service.appSystemCode,
		BusinessImpact:   "Full or partial degradation in serving requests from Elasticsearch",
		Name:             "Check Elasticsearch cluster health",
		PanicGuide:       "https://dewey.in.ft.com/view/system/content-rw-elasticsearch#general",
		Severity:         1,
		TechnicalSummary: "Elasticsearch cluster is not healthy. Details on /__health-details",
		Checker:          service.healthChecker,
	}
}

func (service *healthService) healthChecker() (string, error) {
	output, err := service.esHealthService.GetClusterHealth()
	if err != nil {
		return "Cluster is not healthy: ", err
	} else if output.Status != "green" {
		return "Cluster is not healthy", fmt.Errorf("Cluster is %v", output.Status)
	} else {
		return "Cluster is healthy", nil
	}
}

func (service *healthService) connectivityHealthyCheck() health.Check {
	return health.Check{
		ID:               service.appSystemCode,
		BusinessImpact:   "Could not connect to Elasticsearch",
		Name:             "Check connectivity to the Elasticsearch cluster",
		PanicGuide:       "https://dewey.in.ft.com/view/system/content-rw-elasticsearch#general",
		Severity:         1,
		TechnicalSummary: "Connection to Elasticsearch cluster could not be created. Please check your AWS credentials.",
		Checker:          service.connectivityChecker,
	}
}

func (service *healthService) connectivityChecker() (string, error) {
	_, err := service.esHealthService.GetClusterHealth()
	if err != nil {
		return "Could not connect to elasticsearch", err
	}

	return "Successfully connected to the cluster", nil
}

func (service *healthService) schemaHealthyCheck() health.Check {
	return health.Check{
		ID:               service.appSystemCode,
		BusinessImpact:   "Search results may be inconsistent",
		Name:             "Check Elasticsearch mapping",
		PanicGuide:       "https://dewey.in.ft.com/view/system/content-rw-elasticsearch#general",
		Severity:         1,
		TechnicalSummary: "Elasticsearch mapping does not match expected mapping. Please check index against the reference https://github.com/Financial-Times/content-rw-elasticsearch/blob/master/runtime/referenceSchema.json",
		Checker:          service.schemaChecker,
	}
}

func (service *healthService) schemaChecker() (string, error) {
	output, err := service.esHealthService.GetSchemaHealth()
	if err != nil {
		return "Could not get schema: ", err
	} else if output != "ok" {
		return "Schema is not healthy", fmt.Errorf("Schema is %v", output)
	} else {
		return "Schema is healthy", nil
	}
}

func (service *healthService) checkKafkaProxyConnectivity() health.Check {
	return health.Check{
		ID:               service.appSystemCode,
		BusinessImpact:   "CombinedPostPublication messages can't be read from the queue. Indexing for search won't work.",
		Name:             "Check kafka-proxy connectivity.",
		PanicGuide:       "https://dewey.in.ft.com/view/system/content-rw-elasticsearch#general",
		Severity:         1,
		TechnicalSummary: "Messages couldn't be read from the queue. Check if kafka-proxy is reachable.",
		Checker:          service.consumerInstance.ConnectivityCheck,
	}
}

func (service *healthService) checkConcordanceAPI() health.Check {
	return health.Check{
		ID:               service.appSystemCode,
		BusinessImpact:   "Annotation-related Elasticsearch fields won't be populated",
		Name:             "Public Concordance API Health check",
		PanicGuide:       "https://dewey.in.ft.com/view/system/content-rw-elasticsearch#general",
		Severity:         2,
		TechnicalSummary: "Public Concordance API is not working correctly",
		Checker:          service.concordanceApi.HealthCheck,
	}
}

func (service *healthService) gtgCheck() gtg.Status {
	for _, check := range service.checks {
		if _, err := check.Checker(); err != nil {
			return gtg.Status{GoodToGo: false, Message: err.Error()}
		}
	}
	return gtg.Status{GoodToGo: true}
}

//HealthDetails returns the response from elasticsearch service /__health endpoint - describing the cluster health
func (service *healthService) HealthDetails(writer http.ResponseWriter, req *http.Request) {

	writer.Header().Set("Content-Type", "application/json")
	output, err := service.esHealthService.GetClusterHealth()
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	var response []byte
	response, err = json.Marshal(*output)
	if err != nil {
		response = []byte(err.Error())
	}

	_, err = writer.Write(response)
	if err != nil {
		logger.WithError(err).Error(err.Error())
	}
}
