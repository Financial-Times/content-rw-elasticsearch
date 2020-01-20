package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/content"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/es"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/service/concept"
	"github.com/Financial-Times/go-logger"
	logger2 "github.com/Financial-Times/go-logger/v2"
	consumer "github.com/Financial-Times/message-queue-gonsumer"
	"github.com/dchest/uniuri"
	"github.com/stretchr/stew/slice"
)

const (
	syntheticRequestPrefix   = "SYNTHETIC-REQ-MON"
	transactionIDHeader      = "X-Request-Id"
	blogsAuthority           = "http://api.ft.com/system/FT-LABS-WP"
	articleAuthority         = "http://api.ft.com/system/FTCOM-METHODE"
	videoAuthority           = "http://api.ft.com/system/NEXT-VIDEO-EDITOR"
	originHeader             = "Origin-System-Id"
	methodeOrigin            = "methode-web-pub"
	wordpressOrigin          = "wordpress"
	videoOrigin              = "next-video-editor"
	pacOrigin                = "http://cmdb.ft.com/systems/pac"
	contentTypeHeader        = "Content-Type"
	audioContentTypeHeader   = "ft-upp-audio"
	articleContentTypeHeader = "ft-upp-article"
)

// Empty type added for older content. Placeholders - which are subject of exclusion - have type Content.
var allowedTypes = []string{"Article", "Video", "MediaResource", "Audio", ""}

type MessageHandler struct {
	esService         es.ServiceI
	messageConsumer   consumer.MessageConsumer
	ConceptGetter     concept.ConceptGetter
	connectToESClient func(config es.AccessConfig, c *http.Client) (es.ClientI, error)
	baseApiUrl        string
	wg                *sync.WaitGroup
	log               *logger2.UPPLogger
}

func NewMessageHandler(service es.ServiceI, conceptGetter concept.ConceptGetter, client *http.Client, queueConfig consumer.QueueConfig, wg *sync.WaitGroup, connectToClient func(config es.AccessConfig, c *http.Client) (es.ClientI, error), log *logger2.UPPLogger) *MessageHandler {
	indexer := &MessageHandler{
		esService:         service,
		ConceptGetter:     conceptGetter,
		connectToESClient: connectToClient,
		wg:                wg,
		log:               log,
	}
	indexer.messageConsumer = consumer.NewConsumer(queueConfig, indexer.handleMessage, client, log)
	return indexer
}

func (handler *MessageHandler) Start(baseApiUrl string, accessConfig es.AccessConfig, httpClient *http.Client) {
	handler.baseApiUrl = baseApiUrl
	channel := make(chan es.ClientI)
	go func() {
		defer close(channel)
		for {
			ec, err := handler.connectToESClient(accessConfig, httpClient)
			if err == nil {
				logger.Info("Connected to Elasticsearch")
				channel <- ec
				return
			}
			logger.Error("Could not connect to Elasticsearch")
			time.Sleep(time.Minute)
		}
	}()
	handler.wg.Add(1)
	go func() {
		defer handler.wg.Done()
		for ec := range channel {
			handler.esService.SetClient(ec)
			//this is a blocking method
			handler.messageConsumer.Start()
		}
	}()
}

func (handler *MessageHandler) Stop() {
	if handler.messageConsumer != nil {
		handler.messageConsumer.Stop()
	}
}

func (handler *MessageHandler) handleMessage(msg consumer.Message) {
	tid := msg.Headers[transactionIDHeader]
	if tid == "" {
		tid = "tid_" + uniuri.NewLen(10) + "_content-rw-elasticsearch"
		logger.WithTransactionID(tid).Info("Generated tid")
	}

	if strings.Contains(tid, syntheticRequestPrefix) {
		logger.WithTransactionID(tid).Info("Ignoring synthetic message")
		return
	}

	var combinedPostPublicationEvent content.EnrichedContent
	err := json.Unmarshal([]byte(msg.Body), &combinedPostPublicationEvent)
	if err != nil {
		logger.WithTransactionID(tid).WithError(err).Error("Cannot unmarshal message body")
		return
	}

	if combinedPostPublicationEvent.Content.BodyXML != "" && combinedPostPublicationEvent.Content.Body == "" {
		combinedPostPublicationEvent.Content.Body = combinedPostPublicationEvent.Content.BodyXML
		combinedPostPublicationEvent.Content.BodyXML = ""
	}

	if !slice.ContainsString(allowedTypes, combinedPostPublicationEvent.Content.Type) {
		logger.WithTransactionID(tid).Infof("Ignoring message of type %s", combinedPostPublicationEvent.Content.Type)
		return
	}

	uuid := combinedPostPublicationEvent.UUID
	logger.WithTransactionID(tid).WithUUID(uuid).Info("Processing combined post publication event")

	var contentType string
	typeHeader := msg.Headers[contentTypeHeader]
	if strings.Contains(typeHeader, audioContentTypeHeader) {
		contentType = AudioType
	} else if strings.Contains(typeHeader, articleContentTypeHeader) {
		contentType = ArticleType
	} else {
		for _, identifier := range combinedPostPublicationEvent.Content.Identifiers {
			if strings.HasPrefix(identifier.Authority, blogsAuthority) {
				contentType = BlogType
			} else if strings.HasPrefix(identifier.Authority, articleAuthority) {
				contentType = ArticleType
			} else if strings.HasPrefix(identifier.Authority, videoAuthority) {
				contentType = VideoType
			}
		}
	}

	if contentType == "" {
		origin := msg.Headers[originHeader]
		if strings.Contains(origin, methodeOrigin) {
			contentType = ArticleType
		} else if strings.Contains(origin, wordpressOrigin) {
			contentType = BlogType
		} else if strings.Contains(origin, videoOrigin) {
			contentType = VideoType
		} else if origin != pacOrigin {
			logger.WithTransactionID(tid).WithUUID(uuid).WithError(err).Error("Failed to index content. Could not infer type of content")
			return
		}
	}

	if combinedPostPublicationEvent.MarkedDeleted == "true" {
		_, err = handler.esService.DeleteData(ContentTypeMap[contentType].Collection, uuid)
		if err != nil {
			logger.WithTransactionID(tid).WithUUID(uuid).WithError(err).Error("Failed to delete indexed content")
			return
		}
		logger.WithMonitoringEvent("ContentDeleteElasticsearch", tid, contentType).WithUUID(uuid).Info("Successfully deleted")
		return
	}

	if combinedPostPublicationEvent.Content.UUID == "" || contentType == "" {
		logger.WithTransactionID(tid).WithUUID(combinedPostPublicationEvent.UUID).Info("Ignoring message with no content")
		return
	}

	payload := handler.ToIndexModel(combinedPostPublicationEvent, contentType, tid)

	_, err = handler.esService.WriteData(ContentTypeMap[contentType].Collection, uuid, payload)
	if err != nil {
		logger.WithTransactionID(tid).WithUUID(uuid).WithError(err).Error("Failed to index content")
		return
	}
	logger.WithMonitoringEvent("ContentWriteElasticsearch", tid, contentType).WithUUID(uuid).Info("Successfully saved")

}
