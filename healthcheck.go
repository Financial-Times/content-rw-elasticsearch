package main

import (
	"encoding/json"
	"fmt"
	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
	log "github.com/Sirupsen/logrus"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

// ResponseOK Successful healthcheck response
const ResponseOK = "OK"

type healthService struct {
	esHealthService esHealthServiceI
	topic           string
	proxyAddress    string
	httpClient      *http.Client
	checks          []health.Check
}

func newHealthService(esHealthService esHealthServiceI, topic string, proxyAddress string) *healthService {
	service := &healthService{
		esHealthService: esHealthService,
		topic:           topic,
		proxyAddress:    proxyAddress,
		httpClient: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConnsPerHost:   20,
				TLSHandshakeTimeout:   3 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}}
	service.checks = []health.Check{
		service.clusterIsHealthyCheck(),
		service.connectivityHealthyCheck(),
		service.schemaHealthyCheck(),
		service.topicHealthcheck()}
	return service
}

func (service *healthService) clusterIsHealthyCheck() health.Check {
	return health.Check{
		BusinessImpact:   "Full or partial degradation in serving requests from Elasticsearch",
		Name:             "Check Elasticsearch cluster health",
		PanicGuide:       "https://dewey.ft.com/content-rw-elasticsearch.html",
		Severity:         1,
		TechnicalSummary: "Elasticsearch cluster is not healthy. Details on /__health-details",
		Checker:          service.healthChecker,
	}
}

func (service *healthService) healthChecker() (string, error) {
	output, err := service.esHealthService.getClusterHealth()
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
		BusinessImpact:   "Could not connect to Elasticsearch",
		Name:             "Check connectivity to the Elasticsearch cluster",
		PanicGuide:       "https://dewey.ft.com/content-rw-elasticsearch.html",
		Severity:         1,
		TechnicalSummary: "Connection to Elasticsearch cluster could not be created. Please check your AWS credentials.",
		Checker:          service.connectivityChecker,
	}
}

func (service *healthService) connectivityChecker() (string, error) {
	_, err := service.esHealthService.getClusterHealth()
	if err != nil {
		return "Could not connect to elasticsearch", err
	}

	return "Successfully connected to the cluster", nil
}

func (service *healthService) schemaHealthyCheck() health.Check {
	return health.Check{
		BusinessImpact:   "Search results may be inconsistent",
		Name:             "Check Elasticsearch mapping",
		PanicGuide:       "https://dewey.ft.com/content-rw-elasticsearch.html",
		Severity:         1,
		TechnicalSummary: "Elasticsearch mapping does not match expected mapping. Please check index against the reference https://github.com/Financial-Times/content-rw-elasticsearch/blob/master/runtime/referenceSchema.json",
		Checker:          service.schemaChecker,
	}
}

func (service *healthService) schemaChecker() (string, error) {
	output, err := service.esHealthService.getSchemaHealth()
	if err != nil {
		return "Could not get schema: ", err
	} else if output != "ok" {
		return "Schema is not healthy", fmt.Errorf("Schema is %v", output)
	} else {
		return "Schema is healthy", nil
	}
}

func (service *healthService) topicHealthcheck() health.Check {
	return health.Check{
		BusinessImpact:   "CombinedPostPublication messages can't be read from the queue. Indexing for search won't work.",
		Name:             fmt.Sprintf("Check kafka-proxy connectivity and %s topic", service.topic),
		PanicGuide:       "https://dewey.ft.com/content-rw-elasticsearch.html",
		Severity:         1,
		TechnicalSummary: "Messages couldn't be read from the queue. Check if kafka-proxy is reachable and topic is present.",
		Checker:          service.checkIfCombinedPublicationTopicIsPresent,
	}
}

func (service *healthService) checkIfCombinedPublicationTopicIsPresent() (string, error) {
	return ResponseOK, service.checkIfTopicIsPresent(service.topic)
}

func (service *healthService) checkIfTopicIsPresent(searchedTopic string) error {

	urlStr := service.proxyAddress + "/__kafka-rest-proxy/topics"

	body, _, err := executeHTTPRequest(urlStr, service.httpClient)
	if err != nil {
		log.Errorf("Healthcheck: %v", err.Error())
		return err
	}

	var topics []string

	err = json.Unmarshal(body, &topics)
	if err != nil {
		log.Errorf("Connection could be established to kafka-proxy, but a parsing error occurred and topic could not be found. %v", err.Error())
		return err
	}

	for _, topic := range topics {
		if topic == searchedTopic {
			return nil
		}
	}

	return fmt.Errorf("Connection could be established to kafka-proxy, but topic %s was not found", searchedTopic)
}

func executeHTTPRequest(urlStr string, httpClient *http.Client) (b []byte, status int, err error) {

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, -1, fmt.Errorf("Error creating requests for url=%s, error=%v", urlStr, err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("Error executing requests for url=%s, error=%v", urlStr, err)
	}

	defer cleanUp(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("Connecting to %s was not successful. Status: %d", urlStr, resp.StatusCode)
	}

	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, http.StatusOK, fmt.Errorf("Could not parse payload from response for url=%s, error=%v", urlStr, err)
	}

	return b, http.StatusOK, err
}

func cleanUp(resp *http.Response) {

	_, err := io.Copy(ioutil.Discard, resp.Body)
	if err != nil {
		log.Warningf("[%v]", err)
	}

	err = resp.Body.Close()
	if err != nil {
		log.Warningf("[%v]", err)
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

	output, err := service.esHealthService.getClusterHealth()
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
		log.Errorf(err.Error())
	}
}
