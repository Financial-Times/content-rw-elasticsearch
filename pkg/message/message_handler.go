package message

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Financial-Times/content-rw-elasticsearch/pkg/config"
	"github.com/Financial-Times/content-rw-elasticsearch/pkg/mapper"
	"github.com/Financial-Times/content-rw-elasticsearch/pkg/schema"

	"github.com/Financial-Times/content-rw-elasticsearch/pkg/es"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/dchest/uniuri"

	"github.com/stretchr/stew/slice"
)

const (
	syntheticRequestPrefix   = "SYNTHETIC-REQ-MON"
	transactionIDHeader      = "X-Request-Id"
	originHeader             = "Origin-System-Id"
	contentTypeHeader        = "Content-Type"
	audioContentTypeHeader   = "ft-upp-audio"
	articleContentTypeHeader = "ft-upp-article"
)

// Empty type added for older content. Placeholders - which are subject of exclusion - have type Content.
var allowedTypes = []string{"Article", "Video", "MediaResource", "Audio", ""}

type ESClient func(config es.AccessConfig, c *http.Client) (es.Client, error)

type Handler struct {
	esService       es.Service
	messageConsumer consumer.MessageConsumer
	Mapper          *mapper.Handler
	esClient        ESClient
	wg              *sync.WaitGroup
	mu              sync.Mutex
	log             *logger.UPPLogger
}

func NewMessageHandler(service es.Service, mapper *mapper.Handler, client *http.Client, queueConfig consumer.QueueConfig, wg *sync.WaitGroup, esClient ESClient, logger *logger.UPPLogger) *Handler {
	indexer := &Handler{esService: service, Mapper: mapper, esClient: esClient, wg: wg, log: logger}
	indexer.messageConsumer = consumer.NewConsumer(queueConfig, indexer.handleMessage, client)
	return indexer
}

func (h *Handler) Start(baseAPIURL string, accessConfig es.AccessConfig, httpClient *http.Client) {
	h.Mapper.BaseAPIURL = baseAPIURL
	channel := make(chan es.Client)
	go func() {
		defer close(channel)
		for {
			ec, err := h.esClient(accessConfig, httpClient)
			if err == nil {
				h.log.Info("Connected to Elasticsearch")
				channel <- ec
				return
			}
			h.log.Error("Could not connect to Elasticsearch")
			time.Sleep(time.Minute)
		}
	}()

	go func() {
		defer h.wg.Done()
		for ec := range channel {
			h.mu.Lock()
			h.wg.Add(1)
			h.mu.Unlock()
			h.esService.SetClient(ec)
			h.startMessageConsumer()
		}
	}()
}

func (h *Handler) Stop() {
	h.mu.Lock()
	if h.messageConsumer != nil {
		h.messageConsumer.Stop()
	}
	h.mu.Unlock()

}

func (h *Handler) startMessageConsumer() {
	// this is a blocking method
	h.messageConsumer.Start()
}

func (h *Handler) handleMessage(msg consumer.Message) {
	tid := msg.Headers[transactionIDHeader]
	if tid == "" {
		tid = "tid_" + uniuri.NewLen(10) + "_content-rw-elasticsearch"
		h.log.WithTransactionID(tid).Info("Generated tid")
	}

	if strings.Contains(tid, syntheticRequestPrefix) {
		h.log.WithTransactionID(tid).Info("Ignoring synthetic message")
		return
	}

	var combinedPostPublicationEvent schema.EnrichedContent
	err := json.Unmarshal([]byte(msg.Body), &combinedPostPublicationEvent)
	if err != nil {
		h.log.WithTransactionID(tid).WithError(err).Error("Cannot unmarshal message body")
		return
	}

	if combinedPostPublicationEvent.Content.BodyXML != "" && combinedPostPublicationEvent.Content.Body == "" {
		combinedPostPublicationEvent.Content.Body = combinedPostPublicationEvent.Content.BodyXML
		combinedPostPublicationEvent.Content.BodyXML = ""
	}

	if !slice.ContainsString(allowedTypes, combinedPostPublicationEvent.Content.Type) {
		h.log.WithTransactionID(tid).Infof("Ignoring message of type %s", combinedPostPublicationEvent.Content.Type)
		return
	}

	uuid := combinedPostPublicationEvent.UUID
	h.log.WithTransactionID(tid).WithUUID(uuid).Info("Processing combined post publication event")

	var contentType string
	typeHeader := msg.Headers[contentTypeHeader]
	if strings.Contains(typeHeader, audioContentTypeHeader) {
		contentType = config.AudioType
	} else if strings.Contains(typeHeader, articleContentTypeHeader) {
		contentType = config.ArticleType
	} else {
		for _, identifier := range combinedPostPublicationEvent.Content.Identifiers {
			if strings.HasPrefix(identifier.Authority, h.Mapper.Config.Authorities.Get(config.BlogType)) {
				contentType = config.BlogType
			} else if strings.HasPrefix(identifier.Authority, h.Mapper.Config.Authorities.Get(config.ArticleType)) {
				contentType = config.ArticleType
			} else if strings.HasPrefix(identifier.Authority, h.Mapper.Config.Authorities.Get(config.VideoType)) {
				contentType = config.VideoType
			}
		}
	}

	if contentType == "" {
		originHeader := msg.Headers[originHeader]
		origins := h.Mapper.Config.Origins

		if strings.Contains(originHeader, origins.Get("methode")) {
			contentType = config.ArticleType
		} else if strings.Contains(originHeader, origins.Get("wordpress")) {
			contentType = config.BlogType
		} else if strings.Contains(originHeader, origins.Get("video")) {
			contentType = config.VideoType
		} else if originHeader != origins.Get("pac") {
			h.log.WithTransactionID(tid).WithUUID(uuid).WithError(err).Error("Failed to index content. Could not infer type of content")
			return
		}
	}

	conceptType := h.Mapper.Config.ContentTypeMap.Get(contentType).Collection
	if combinedPostPublicationEvent.MarkedDeleted == "true" {
		_, err = h.esService.DeleteData(conceptType, uuid)
		if err != nil {
			h.log.WithTransactionID(tid).WithUUID(uuid).WithError(err).Error("Failed to delete indexed content")
			return
		}
		h.log.WithMonitoringEvent("ContentDeleteElasticsearch", tid, contentType).WithUUID(uuid).Info("Successfully deleted")
		return
	}

	if combinedPostPublicationEvent.Content.UUID == "" || contentType == "" {
		h.log.WithTransactionID(tid).WithUUID(combinedPostPublicationEvent.UUID).Info("Ignoring message with no content")
		return
	}

	payload := h.Mapper.ToIndexModel(combinedPostPublicationEvent, contentType, tid)
	h.log.Info(conceptType)

	_, err = h.esService.WriteData(conceptType, uuid, payload)
	if err != nil {
		h.log.WithTransactionID(tid).WithUUID(uuid).WithError(err).Error("Failed to index content")
		return
	}
	h.log.WithMonitoringEvent("ContentWriteElasticsearch", tid, contentType).WithUUID(uuid).Info("Successfully saved")
}
