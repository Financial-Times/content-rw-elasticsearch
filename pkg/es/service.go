package es

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/Financial-Times/content-rw-elasticsearch/pkg/config"

	"gopkg.in/olivere/elastic.v2"
)

var referenceIndex *elasticIndex

type elasticIndex struct {
	index map[string]*elastic.IndicesGetResponse
}

type ElasticsearchService struct {
	sync.RWMutex
	ElasticClient Client
	IndexName     string
}

type Service interface {
	HealthStatus
	SetClient(client Client)
	WriteData(conceptType string, uuid string, payload interface{}) (*elastic.IndexResult, error)
	DeleteData(conceptType string, uuid string) (*elastic.DeleteResult, error)
}

type HealthStatus interface {
	GetClusterHealth() (*elastic.ClusterHealthResponse, error)
	GetSchemaHealth() (string, error)
}

func NewService(indexName string) Service {
	return &ElasticsearchService{IndexName: indexName}
}

func (s *ElasticsearchService) GetClusterHealth() (*elastic.ClusterHealthResponse, error) {
	if s.ElasticClient == nil {
		return nil, errors.New("client could not be created, please check the application parameters/env variables, and restart the service")
	}

	return s.ElasticClient.ClusterHealth().Do()
}

func (s *ElasticsearchService) GetSchemaHealth() (string, error) {
	if referenceIndex == nil {
		referenceIndex = new(elasticIndex)

		referenceJSON, err := config.ReadEmbeddedResource("referenceSchema.json")
		if err != nil {
			return "", err
		}

		fullReferenceJSON := []byte(fmt.Sprintf(`{"ft": %s}`, string(referenceJSON)))
		err = json.Unmarshal(fullReferenceJSON, &referenceIndex.index)
		if err != nil {
			return "", err
		}
	}
	if referenceIndex.index[s.IndexName] == nil || referenceIndex.index[s.IndexName].Settings == nil || referenceIndex.index[s.IndexName].Mappings == nil {
		return "not ok, wrong referenceIndex", nil
	}

	if s.ElasticClient == nil {
		return "not ok, connection to ES couldn't be established", nil
	}

	liveIndex, err := s.ElasticClient.IndexGet().Index(s.IndexName).Do()
	if err != nil {
		return "", err
	}

	settings, ok := liveIndex[s.IndexName].Settings["index"].(map[string]interface{})
	if ok {
		delete(settings, "creation_date")
		delete(settings, "uuid")
		delete(settings, "version")
		delete(settings, "created")
	}

	if !reflect.DeepEqual(liveIndex[s.IndexName].Settings, referenceIndex.index[s.IndexName].Settings) {
		return "not ok, wrong settings", nil
	}

	if !reflect.DeepEqual(liveIndex[s.IndexName].Mappings, referenceIndex.index[s.IndexName].Mappings) {
		return "not ok, wrong mappings", nil
	}

	return "ok", nil
}

func (s *ElasticsearchService) SetClient(client Client) {
	s.Lock()
	defer s.Unlock()
	s.ElasticClient = client
}

func (s *ElasticsearchService) WriteData(conceptType string, uuid string, payload interface{}) (*elastic.IndexResult, error) {
	return s.ElasticClient.Index().
		Index(s.IndexName).
		Type(conceptType).
		Id(uuid).
		BodyJson(payload).
		Do()
}

func (s *ElasticsearchService) DeleteData(conceptType string, uuid string) (*elastic.DeleteResult, error) {
	return s.ElasticClient.Delete().
		Index(s.IndexName).
		Type(conceptType).
		Id(uuid).
		Do()
}
