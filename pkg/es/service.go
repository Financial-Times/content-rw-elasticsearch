package es

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/config"
	"github.com/olivere/elastic/v7"
)

var referenceIndex *elasticIndex

type elasticIndex struct {
	index map[string]*elastic.IndicesGetResponse
}

type ElasticsearchService struct {
	mu            sync.RWMutex
	ElasticClient Client
	IndexName     string
}

type Service interface {
	HealthStatus
	SetClient(client Client)
	WriteData(ctx context.Context, uuid string, payload interface{}) (*elastic.IndexResponse, error)
	DeleteData(ctx context.Context, uuid string) (*elastic.DeleteResponse, error)
}

type HealthStatus interface {
	GetClusterHealth(ctx context.Context) (*elastic.ClusterHealthResponse, error)
	GetSchemaHealth(ctx context.Context) (string, error)
}

func NewService(indexName string) Service {
	return &ElasticsearchService{IndexName: indexName}
}

func (s *ElasticsearchService) GetClusterHealth(ctx context.Context) (*elastic.ClusterHealthResponse, error) {
	if s.ElasticClient == nil {
		return nil, errors.New("client could not be created, please check the application parameters/env variables, and restart the service")
	}

	return s.ElasticClient.ClusterHealth().Do(ctx)
}

func (s *ElasticsearchService) GetSchemaHealth(ctx context.Context) (string, error) {
	if referenceIndex == nil {
		referenceIndex = new(elasticIndex)

		referenceJSON, err := config.ReadConfigFile("referenceSchema.json")
		if err != nil {
			return "", err
		}

		fullReferenceJSON := []byte(fmt.Sprintf(`{"%s": %s}`, s.IndexName, string(referenceJSON)))
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

	liveIndex, err := s.ElasticClient.IndexGet().Index(s.IndexName).Do(ctx)
	if err != nil {
		return "", err
	}

	indices := make([]string, 0, len(liveIndex))
	for indexName := range liveIndex {
		indices = append(indices, indexName)
	}
	if len(indices) < 1 {
		return fmt.Sprintf("not ok, could not find index or alias %s", s.IndexName), nil
	}
	// we are getting the first index name because we are fetching by index or alias but response is real name
	realIndexName := indices[0]

	settings, ok := liveIndex[realIndexName].Settings["index"].(map[string]interface{})
	if ok {
		delete(settings, "creation_date")
		delete(settings, "uuid")
		delete(settings, "version")
		delete(settings, "created")
		delete(settings, "provided_name")
	}

	if !reflect.DeepEqual(liveIndex[realIndexName].Settings, referenceIndex.index[s.IndexName].Settings) {
		return "not ok, wrong settings", nil
	}

	if !reflect.DeepEqual(liveIndex[realIndexName].Mappings, referenceIndex.index[s.IndexName].Mappings) {
		return "not ok, wrong mappings", nil
	}

	return "ok", nil
}

func (s *ElasticsearchService) GetClient() Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ElasticClient
}
func (s *ElasticsearchService) SetClient(client Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ElasticClient = client
}

func (s *ElasticsearchService) WriteData(ctx context.Context, uuid string, payload interface{}) (*elastic.IndexResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ElasticClient.Index().
		Index(s.IndexName).
		Id(uuid).
		BodyJson(payload).
		Do(ctx)
}

func (s *ElasticsearchService) DeleteData(ctx context.Context, uuid string) (*elastic.DeleteResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ElasticClient.Delete().
		Index(s.IndexName).
		Id(uuid).
		Do(ctx)
}
