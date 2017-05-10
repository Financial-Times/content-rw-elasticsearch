package main

import (
	"encoding/json"
	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	log "github.com/Sirupsen/logrus"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	healthPath             = "/__health"
	healthDetailsPath      = "/__health-details"
	syntheticRequestPrefix = "SYNTHETIC-REQ-MON"
	transactionIDHeader    = "X-Request-Id"
	blogsAuthority         = "http://api.ft.com/system/FT-LABS-WP"
	articleAuthority       = "http://api.ft.com/system/FTCOM-METHODE"
	videoAuthority         = "http://api.ft.com/system/NEXT-VIDEO-EDITOR"
	originHeader           = "Origin-System-Id"
	methodeOrigin          = "methode-web-pub"
	wordpressOrigin        = "wordpress"
	videoOrigin            = "next-video-editor"
	blogPostType           = "blogPost"
	articleType            = "article"
	videoType              = "video"
)

// Empty type added for older content. Placeholders - which are subject of exclusion - have type Content.
var allowedTypes = []string{"Article", "Video", "MediaResource", ""}

type contentIndexer struct {
	esServiceInstance esServiceI
	server            *http.Server
	messageConsumer   consumer.MessageConsumer
	wg                sync.WaitGroup
	mu                sync.Mutex
}

func (indexer *contentIndexer) start(appSystemCode string, appName string, indexName string, port string, accessConfig esAccessConfig, queueConfig consumer.QueueConfig) {
	channel := make(chan esClientI)
	go func() {
		defer close(channel)
		for {
			ec, err := newAmazonClient(accessConfig)
			if err == nil {
				log.Infof("connected to Elasticsearch")
				channel <- ec
				return
			}
			log.Errorf("could not connect to Elasticsearch: %s", err.Error())
			time.Sleep(time.Minute)
		}
	}()

	//create writer service
	indexer.esServiceInstance = newEsService(indexName)

	go func() {
		for ec := range channel {
			indexer.esServiceInstance.setClient(ec)
			indexer.startMessageConsumer(queueConfig)
		}
	}()

	indexer.serveAdminEndpoints(appSystemCode, appName, port, queueConfig)
}

func (indexer *contentIndexer) stop() {
	go func() {
		indexer.mu.Lock()
		if indexer.messageConsumer != nil {
			indexer.messageConsumer.Stop()
		}
		indexer.mu.Unlock()
	}()
	if err := indexer.server.Close(); err != nil {
		log.Errorf("Unable to stop http server: %v", err)
	}
	indexer.wg.Wait()
}

func (indexer *contentIndexer) serveAdminEndpoints(appSystemCode string, appName string, port string, queueConfig consumer.QueueConfig) {
	healthService := newHealthService(indexer.esServiceInstance, queueConfig.Topic, queueConfig.Addrs[0])

	serveMux := http.NewServeMux()

	hc := health.HealthCheck{SystemCode: appSystemCode, Name: appName, Description: "Content Read Writer for Elasticsearch", Checks: healthService.checks}
	serveMux.HandleFunc(healthPath, health.Handler(hc))
	serveMux.HandleFunc(healthDetailsPath, healthService.HealthDetails)
	serveMux.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.gtgCheck))
	serveMux.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	indexer.server = &http.Server{Addr: ":" + port, Handler: serveMux}

	indexer.wg.Add(1)
	go func() {
		if err := indexer.server.ListenAndServe(); err != nil {
			log.Infof("HTTP server closing with message: %v", err)
		}
		indexer.wg.Done()
	}()
}

func (indexer *contentIndexer) startMessageConsumer(config consumer.QueueConfig) {
	client := &http.Client{
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
	}
	indexer.mu.Lock()
	indexer.messageConsumer = consumer.NewConsumer(config, indexer.handleMessage, client)
	indexer.mu.Unlock()

	indexer.wg.Add(1)

	indexer.messageConsumer.Start()
	indexer.wg.Done()
}

func (indexer *contentIndexer) handleMessage(msg consumer.Message) {

	tid := msg.Headers[transactionIDHeader]

	if strings.Contains(tid, syntheticRequestPrefix) {
		log.Infof("[%s] Ignoring synthetic message", tid)
		return
	}

	var combinedPostPublicationEvent enrichedContentModel
	err := json.Unmarshal([]byte(msg.Body), &combinedPostPublicationEvent)
	if err != nil {
		log.Errorf("[%s] Cannot unmarshal message body:[%v]", tid, err.Error())
		return
	}

	if !contains(allowedTypes, combinedPostPublicationEvent.Content.Type) {
		log.Infof("[%s] Ignoring message of type %s", tid, combinedPostPublicationEvent.Content.Type)
		return
	}

	uuid := combinedPostPublicationEvent.Content.UUID
	log.Printf("[%s] Processing combined post publication event for uuid [%s]", tid, uuid)

	var contentType string

	for _, identifier := range combinedPostPublicationEvent.Content.Identifiers {
		if strings.HasPrefix(identifier.Authority, blogsAuthority) {
			contentType = blogPostType
		} else if strings.HasPrefix(identifier.Authority, articleAuthority) {
			contentType = articleType
		} else if strings.HasPrefix(identifier.Authority, videoAuthority) {
			contentType = videoType
		}
	}

	if contentType == "" {
		origin := msg.Headers[originHeader]
		if strings.Contains(origin, methodeOrigin) {
			contentType = articleType
		} else if strings.Contains(origin, wordpressOrigin) {
			contentType = blogPostType
		} else if strings.Contains(origin, videoOrigin) {
			contentType = videoType
		} else {
			log.Errorf("[%s] Failed to index content with UUID %s. Could not infer type of content.", tid, uuid)
			return
		}
	}

	if combinedPostPublicationEvent.Content.MarkedDeleted {
		_, err = indexer.esServiceInstance.deleteData(contentTypeMap[contentType].collection, uuid)
		if err != nil {
			log.Errorf(err.Error())
			return
		}
	} else {
		payload := convertToESContentModel(combinedPostPublicationEvent, contentType)

		_, err = indexer.esServiceInstance.writeData(contentTypeMap[contentType].collection, uuid, payload)
		if err != nil {
			log.Errorf(err.Error())
			return
		}
	}
}

func contains(list []string, elem string) bool {
	for _, a := range list {
		if a == elem {
			return true
		}
	}
	return false
}
