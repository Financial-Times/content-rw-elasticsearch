package es

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"sync"

	"gopkg.in/olivere/elastic.v2"
)

var referenceIndex *elasticIndex

type elasticIndex struct {
	index map[string]*elastic.IndicesGetResponse
}

type Service struct {
	sync.RWMutex
	ElasticClient ClientI
	IndexName     string
}

type ServiceI interface {
	HealthServiceI
	SetClient(client ClientI)
	WriteDataInBulk(conceptType string, uuid string, payload interface{}) (*elastic.BulkResponse, error)
	WriteData(conceptType string, uuid string, payload interface{}) (*elastic.IndexResult, error)
	DeleteData(conceptType string, uuid string) (*elastic.DeleteResult, error)
}

type HealthServiceI interface {
	GetClusterHealth() (*elastic.ClusterHealthResponse, error)
	GetSchemaHealth() (string, error)
}

func NewService(indexName string) *Service {
	return &Service{IndexName: indexName}
}

func (service *Service) GetClusterHealth() (*elastic.ClusterHealthResponse, error) {
	if service.ElasticClient == nil {
		return nil, errors.New("client could not be created, please check the application parameters/env variables, and restart the service")
	}

	return service.ElasticClient.ClusterHealth().Do()
}

func (service *Service) GetSchemaHealth() (string, error) {

	if referenceIndex == nil {
		referenceIndex = new(elasticIndex)

		referenceJSON, err := ioutil.ReadFile("runtime/referenceSchema.json")
		if err != nil {
			return "", err
		}

		err = json.Unmarshal([]byte(fmt.Sprintf(`{"ft": %s}`, referenceJSON)), &referenceIndex.index)
		if err != nil {
			return "", err
		}
	}

	if service.ElasticClient == nil {
		return "not ok, connection to ES couldn't be established", nil
	}

	liveIndex, err := service.ElasticClient.IndexGet().Index(service.IndexName).Do()
	if err != nil {
		return "", err
	}

	settings, ok := liveIndex[service.IndexName].Settings["index"].(map[string]interface{})
	if ok {
		delete(settings, "creation_date")
		delete(settings, "uuid")
		delete(settings, "version")
		delete(settings, "created")
	}

	if !reflect.DeepEqual(liveIndex[service.IndexName].Settings, referenceIndex.index[service.IndexName].Settings) {
		return "not ok, wrong settings", nil
	}

	if !reflect.DeepEqual(liveIndex[service.IndexName].Mappings, referenceIndex.index[service.IndexName].Mappings) {
		return "not ok, wrong mappings", nil
	}

	return "ok", nil
}

func (service *Service) SetClient(client ClientI) {
	service.Lock()
	defer service.Unlock()
	service.ElasticClient = client
}

func (service *Service) WriteDataInBulk(conceptType string, uuid string, payload interface{}) (*elastic.BulkResponse, error) {
	req := elastic.NewBulkIndexRequest().Index(service.IndexName).
		Type(conceptType).
		Id(uuid).
		Doc(payload)

	return service.ElasticClient.Bulk().
		Add(req).
		Do()
}

func (service *Service) WriteData(conceptType string, uuid string, payload interface{}) (*elastic.IndexResult, error) {
	return service.ElasticClient.Index().
		Index(service.IndexName).
		Type(conceptType).
		Id(uuid).
		BodyJson(payload).
		Do()
}

func (service *Service) DeleteData(conceptType string, uuid string) (*elastic.DeleteResult, error) {
	return service.ElasticClient.Delete().
		Index(service.IndexName).
		Type(conceptType).
		Id(uuid).
		Do()
}
