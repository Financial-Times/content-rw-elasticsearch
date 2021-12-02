package health

import (
	"encoding/json"
	"fmt"
	"net/http"

	status "github.com/Financial-Times/service-status-go/httphandlers"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/concept"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/es"
	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/Financial-Times/service-status-go/gtg"
)

const (
	pathHealth        = "/__health"
	pathHealthDetails = "/__health-details"
	panicGuide        = "https://runbooks.in.ft.com/content-rw-elasticsearch"
)

type Service struct {
	ESHealthService  es.HealthStatus
	ConcordanceAPI   *concept.ConcordanceAPIService
	ConsumerInstance consumer.MessageConsumer
	HTTPClient       *http.Client
	Checks           []fthealth.Check
	AppSystemCode    string
	log              *logger.UPPLogger
}

func NewHealthService(config *consumer.QueueConfig, esHealthService es.HealthStatus, client *http.Client, concordanceAPI *concept.ConcordanceAPIService, appSystemCode string, log *logger.UPPLogger) *Service {
	consumerInstance := consumer.NewConsumer(*config, func(m consumer.Message) {}, client)
	service := &Service{
		ESHealthService:  esHealthService,
		ConcordanceAPI:   concordanceAPI,
		ConsumerInstance: consumerInstance,
		HTTPClient:       client,
		AppSystemCode:    appSystemCode,
		log:              log,
	}
	service.Checks = []fthealth.Check{
		service.clusterIsHealthyCheck(),
		service.connectivityHealthyCheck(),
		service.schemaHealthyCheck(),
		service.checkKafkaProxyConnectivity(),
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
	output, err := s.ESHealthService.GetClusterHealth()
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
	_, err := s.ESHealthService.GetClusterHealth()
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
		Checker: func() (string, error) {
			status, err := s.schemaChecker()
			if err != nil {
				s.log.WithError(err).WithField("status", status).Error("Error verifying schema")
			} else {
				s.log.WithField("status", status).Info("Successfully verified schema")
			}

			return status, err
		},
	}
}

func (s *Service) schemaChecker() (string, error) {
	output, err := s.ESHealthService.GetSchemaHealth()
	if err != nil {
		return "Could not get schema: ", err
	} else if output != "ok" {
		return "Schema is not healthy", fmt.Errorf("schema is %v", output)
	} else {
		return "Schema is healthy", nil
	}
}

func (s *Service) checkKafkaProxyConnectivity() fthealth.Check {
	return fthealth.Check{
		ID:               s.AppSystemCode,
		BusinessImpact:   "CombinedPostPublication messages can't be read from the queue. Indexing for search won't work.",
		Name:             "Check kafka-proxy connectivity.",
		PanicGuide:       panicGuide,
		Severity:         1,
		TechnicalSummary: "Messages couldn't be read from the queue. Check if kafka-proxy is reachable.",
		Checker:          s.ConsumerInstance.ConnectivityCheck,
	}
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
	output, err := s.ESHealthService.GetClusterHealth()
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
