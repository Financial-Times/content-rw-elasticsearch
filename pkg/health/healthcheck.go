package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/concept"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/es"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/message"
	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/service-status-go/gtg"
	status "github.com/Financial-Times/service-status-go/httphandlers"
)

const (
	pathHealth        = "/__health"
	pathHealthDetails = "/__health-details"
	panicGuide        = "https://runbooks.in.ft.com/content-rw-elasticsearch"

	contextTimeout = 5 * time.Second
)

type Service struct {
	ESHealthService  es.HealthStatus
	ConcordanceAPI   *concept.ConcordanceAPIService
	ConsumerInstance message.Consumer
	HTTPClient       *http.Client
	Checks           []fthealth.Check
	AppSystemCode    string
	log              *logger.UPPLogger
}

func NewHealthService(consumer message.Consumer, esHealthService es.HealthStatus, client *http.Client, concordanceAPI *concept.ConcordanceAPIService, appSystemCode string, log *logger.UPPLogger) *Service {
	service := &Service{
		ESHealthService:  esHealthService,
		ConcordanceAPI:   concordanceAPI,
		ConsumerInstance: consumer,
		HTTPClient:       client,
		AppSystemCode:    appSystemCode,
		log:              log,
	}
	service.Checks = []fthealth.Check{
		service.clusterIsHealthyCheck(),
		service.connectivityHealthyCheck(),
		service.schemaHealthyCheck(),
		service.kafkaConnectivityCheck(),
		service.kafkaMonitoringCheck(),
		service.checkConcordanceAPI(),
	}
	return service
}

func (s *Service) AttachHTTPEndpoints(serveMux *http.ServeMux, appName string, appDescription string) *http.ServeMux {
	hc := fthealth.HealthCheck{
		SystemCode:  s.AppSystemCode,
		Name:        appName,
		Description: appDescription,
		Checks:      s.Checks,
	}
	serveMux.HandleFunc(pathHealth, fthealth.Handler(hc))
	serveMux.HandleFunc(pathHealthDetails, s.healthDetails)
	serveMux.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(s.gtgCheck))
	serveMux.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	return serveMux
}

func (s *Service) clusterIsHealthyCheck() fthealth.Check {
	return fthealth.Check{
		ID:               s.AppSystemCode,
		BusinessImpact:   "Full or partial degradation in serving requests from Elasticsearch",
		Name:             "Check Elasticsearch cluster health",
		PanicGuide:       panicGuide,
		Severity:         1,
		TechnicalSummary: "Elasticsearch cluster is not healthy. Details on /__health-details",
		Checker:          s.healthChecker,
	}
}

func (s *Service) healthChecker() (string, error) {
	ctx, ctxClose := context.WithTimeout(context.Background(), contextTimeout)
	defer ctxClose()
	output, err := s.ESHealthService.GetClusterHealth(ctx)
	if err != nil {
		return "Cluster is not healthy: ", err
	} else if output.Status != "green" {
		return "Cluster is not healthy", fmt.Errorf("cluster is %v", output.Status)
	} else {
		return "Cluster is healthy", nil
	}
}

func (s *Service) connectivityHealthyCheck() fthealth.Check {
	return fthealth.Check{
		ID:               s.AppSystemCode,
		BusinessImpact:   "Could not connect to Elasticsearch",
		Name:             "Check connectivity to the Elasticsearch cluster",
		PanicGuide:       panicGuide,
		Severity:         1,
		TechnicalSummary: "Connection to Elasticsearch cluster could not be created. Please check your AWS credentials.",
		Checker:          s.connectivityChecker,
	}
}

func (s *Service) connectivityChecker() (string, error) {
	ctx, ctxClose := context.WithTimeout(context.Background(), contextTimeout)
	defer ctxClose()
	_, err := s.ESHealthService.GetClusterHealth(ctx)
	if err != nil {
		return "Could not connect to elasticsearch", err
	}

	return "Successfully connected to the cluster", nil
}

func (s *Service) schemaHealthyCheck() fthealth.Check {
	return fthealth.Check{
		ID:               s.AppSystemCode,
		BusinessImpact:   "Search results may be inconsistent",
		Name:             "Check Elasticsearch mapping",
		PanicGuide:       "https://runbooks.in.ft.com/content-rw-elasticsearch",
		Severity:         1,
		TechnicalSummary: "Elasticsearch mapping does not match expected mapping. Please check index against the reference https://github.com/Financial-Times/content-rw-elasticsearch/blob/master/configs/referenceSchema.json",
		Checker:          s.schemaChecker,
	}
}

func (s *Service) schemaChecker() (string, error) {
	ctx, ctxClose := context.WithTimeout(context.Background(), contextTimeout)
	defer ctxClose()
	output, err := s.ESHealthService.GetSchemaHealth(ctx)
	if err != nil {
		return "Could not get schema: ", err
	} else if output != "ok" {
		return "Schema is not healthy", fmt.Errorf("schema is %v", output)
	} else {
		return "Schema is healthy", nil
	}
}

func (s *Service) kafkaConnectivityCheck() fthealth.Check {
	return fthealth.Check{
		ID:               s.AppSystemCode,
		BusinessImpact:   "CombinedPostPublication messages can't be read from the queue. Indexing for search won't work.",
		Name:             "Check Kafka connectivity",
		PanicGuide:       panicGuide,
		Severity:         1,
		TechnicalSummary: "Establishing Kafka connection failed. Check if Kafka is reachable.",
		Checker:          s.kafkaConnectivityChecker,
	}
}

func (s *Service) kafkaConnectivityChecker() (string, error) {
	err := s.ConsumerInstance.ConnectivityCheck()
	if err != nil {
		return "", err
	}
	return "Successfully connected to Kafka", nil
}

func (s *Service) kafkaMonitoringCheck() fthealth.Check {
	return fthealth.Check{
		ID:               s.AppSystemCode,
		BusinessImpact:   "Consumer is lagging behind when reading messages. Indexing of content is delayed.",
		Name:             "Check Kafka consumer status",
		PanicGuide:       panicGuide,
		Severity:         3,
		TechnicalSummary: "Messages awaiting handling exceed the configured lag tolerance. Check if Kafka consumer is stuck.",
		Checker:          s.kafkaMonitoringChecker,
	}
}

func (s *Service) kafkaMonitoringChecker() (string, error) {
	if err := s.ConsumerInstance.MonitorCheck(); err != nil {
		return "", err
	}
	return "Kafka consumer status is healthy", nil
}

func (s *Service) checkConcordanceAPI() fthealth.Check {
	return fthealth.Check{
		ID:               s.AppSystemCode,
		BusinessImpact:   "Annotation-related Elasticsearch fields won't be populated",
		Name:             "Public Concordance API Health check",
		PanicGuide:       panicGuide,
		Severity:         2,
		TechnicalSummary: "Public Concordance API is not working correctly",
		Checker:          s.ConcordanceAPI.HealthCheck,
	}
}

func (s *Service) gtgCheck() gtg.Status {
	for _, check := range s.Checks {
		if _, err := check.Checker(); err != nil {
			return gtg.Status{GoodToGo: false, Message: err.Error()}
		}
	}
	return gtg.Status{GoodToGo: true}
}

// HealthDetails returns the response from elasticsearch service /__health endpoint - describing the cluster health
func (s *Service) healthDetails(writer http.ResponseWriter, req *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	ctx, ctxClose := context.WithTimeout(context.Background(), contextTimeout)
	defer ctxClose()
	output, err := s.ESHealthService.GetClusterHealth(ctx)
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
		s.log.WithError(err).Error(err.Error())
	}
}
