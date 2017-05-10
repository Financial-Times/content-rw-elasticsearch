package main

import (
	"encoding/json"
	"errors"
	"gopkg.in/olivere/elastic.v2"
	"io/ioutil"
	"reflect"
	"sync"
)

var referenceIndex *elasticIndex

type elasticIndex struct {
	index map[string]*elastic.IndicesGetResponse
}

type esService struct {
	sync.RWMutex
	elasticClient esClientI
	indexName     string
}

type esServiceI interface {
	esHealthServiceI
	setClient(client esClientI)
	writeData(conceptType string, uuid string, payload interface{}) (*elastic.IndexResult, error)
	deleteData(conceptType string, uuid string) (*elastic.DeleteResult, error)
}

type esHealthServiceI interface {
	getClusterHealth() (*elastic.ClusterHealthResponse, error)
	getSchemaHealth() (string, error)
}

func newEsService(indexName string) esServiceI {
	return &esService{indexName: indexName}
}

func (service *esService) getClusterHealth() (*elastic.ClusterHealthResponse, error) {
	if service.elasticClient == nil {
		return nil, errors.New("client could not be created, please check the application parameters/env variables, and restart the service")
	}

	return service.elasticClient.ClusterHealth().Do()
}

func (service *esService) getSchemaHealth() (string, error) {

	if referenceIndex == nil {
		referenceIndex = new(elasticIndex)

		referenceJSON, err := ioutil.ReadFile("runtime/referenceSchema.json")
		if err != nil {
			return "", err
		}

		err = json.Unmarshal(referenceJSON, &referenceIndex.index)
		if err != nil {
			return "", err
		}
	}

	liveIndex, err := service.elasticClient.IndexGet().Index(service.indexName).Do()
	if err != nil {
		return "", err
	}

	settings, ok := liveIndex[service.indexName].Settings["index"].(map[string]interface{})
	if ok {
		delete(settings, "creation_date")
		delete(settings, "uuid")
		delete(settings, "version")
		delete(settings, "created")
	}

	if !reflect.DeepEqual(liveIndex[service.indexName].Settings, referenceIndex.index[service.indexName].Settings) {
		return "not ok, wrong settings", nil
	}

	if !reflect.DeepEqual(liveIndex[service.indexName].Mappings, referenceIndex.index[service.indexName].Mappings) {
		return "not ok, wrong mappings", nil
	}

	return "ok", nil
}

func (service *esService) setClient(client esClientI) {
	service.Lock()
	defer service.Unlock()
	service.elasticClient = client
}

func (service *esService) writeData(conceptType string, uuid string, payload interface{}) (*elastic.IndexResult, error) {
	return service.elasticClient.Index().
		Index(service.indexName).
		Type(conceptType).
		Id(uuid).
		BodyJson(payload).
		Do()
}

func (service *esService) deleteData(conceptType string, uuid string) (*elastic.DeleteResult, error) {
	return service.elasticClient.Delete().
		Index(service.indexName).
		Type(conceptType).
		Id(uuid).
		Do()
}
