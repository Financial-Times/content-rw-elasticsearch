package message

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/config"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/es"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/mapper"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/schema"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v4"
	transactionid "github.com/Financial-Times/transactionid-utils-go"
)

const (
	syntheticRequestPrefix     = "SYNTHETIC-REQ-MON"
	transactionIDHeader        = "X-Request-Id"
	originHeader               = "Origin-System-Id"
	contentTypeHeader          = "Content-Type"
	audioContentTypeHeader     = "ft-upp-audio"
	articleContentTypeHeader   = "ft-upp-article"
	liveEventContentTypeHeader = "ft-upp-live-event"

	contextTimeout = 10 * time.Second
)

type ESClient func(config es.AccessConfig, c *http.Client, log *logger.UPPLogger) (es.Client, error)

type Consumer interface {
	Start(messageHandler func(message kafka.FTMessage))
	Close() error
	ConnectivityCheck() error
	MonitorCheck() error
}

type Handler struct {
	esService  es.Service
	consumer   Consumer
	mapper     *mapper.Handler
	httpClient *http.Client
	esClient   ESClient
	log        *logger.UPPLogger
}

func NewMessageHandler(service es.Service, mapper *mapper.Handler, httpClient *http.Client, consumer Consumer, esClient ESClient, logger *logger.UPPLogger) *Handler {
	indexer := &Handler{
		esService:  service,
		consumer:   consumer,
		mapper:     mapper,
		httpClient: httpClient,
		esClient:   esClient,
		log:        logger,
	}
	return indexer
}

func (h *Handler) Start(baseAPIURL string, accessConfig es.AccessConfig) {
	h.mapper.BaseAPIURL = baseAPIURL
	go func() {
		for {
			ec, err := h.esClient(accessConfig, h.httpClient, h.log)
			if err != nil {
				h.log.Error("Could not connect to Elasticsearch")
				time.Sleep(time.Minute)
				continue
			}
			h.esService.SetClient(ec)
			h.log.Info("Connected to Elasticsearch")
			// this is a blocking method
			h.consumer.Start(h.handleMessage)
			return
		}
	}()
}

func (h *Handler) Stop() {
	if h.consumer != nil {
		if err := h.consumer.Close(); err != nil {
			h.log.WithError(err).Error("Failed closing consumer connection")
		}
	}
}

func (h *Handler) handleMessage(msg kafka.FTMessage) {
	tid := msg.Headers[transactionIDHeader]
	log := h.log.WithTransactionID(tid)

	if tid == "" {
		tid = transactionid.NewTransactionID()
		log = h.log.WithTransactionID(tid)
		log.Info("Generated tid")
	}

	if strings.Contains(tid, syntheticRequestPrefix) {
		log.Info("Ignoring synthetic message")
		return
	}

	var combinedPostPublicationEvent schema.EnrichedContent
	err := json.Unmarshal([]byte(msg.Body), &combinedPostPublicationEvent)
	if err != nil {
		log.WithError(err).Error("Cannot unmarshal message body")
		return
	}

	if combinedPostPublicationEvent.Content.BodyXML != "" && combinedPostPublicationEvent.Content.Body == "" {
		combinedPostPublicationEvent.Content.Body = combinedPostPublicationEvent.Content.BodyXML
		combinedPostPublicationEvent.Content.BodyXML = ""
	}

	if !isAllowedType(combinedPostPublicationEvent.Content.Type) {
		log.Infof("Ignoring message of type %s", combinedPostPublicationEvent.Content.Type)
		return
	}

	uuid := combinedPostPublicationEvent.UUID
	log = log.WithUUID(uuid)
	log.Info(fmt.Sprintf("Processing combined post publication event with type %s", combinedPostPublicationEvent.Content.Type))

	contentType := h.readContentType(msg, combinedPostPublicationEvent)
	if contentType == "" && msg.Headers[originHeader] != config.PACOrigin {
		log.Error("Failed to index content. Could not infer type of content")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	if combinedPostPublicationEvent.Deleted {
		_, err = h.esService.DeleteData(ctx, uuid)
		if err != nil {
			log.WithError(err).Error("Failed to delete indexed content")
			return
		}
		log.WithMonitoringEvent("ContentDeleteElasticsearch", tid, contentType).Info("Successfully deleted")
		return
	}

	if combinedPostPublicationEvent.Content.UUID == "" || contentType == "" {
		log.Info("Ignoring message with no content")
		return
	}

	payload := h.mapper.ToIndexModel(combinedPostPublicationEvent, contentType, tid)

	_, err = h.esService.WriteData(ctx, uuid, payload)
	if err != nil {
		log.WithError(err).Error("Failed to index content")
		return
	}
	log.WithMonitoringEvent("ContentWriteElasticsearch", tid, contentType).Info("Successfully saved")
}

func (h *Handler) readContentType(msg kafka.FTMessage, event schema.EnrichedContent) string {
	typeHeader := msg.Headers[contentTypeHeader]
	if strings.Contains(typeHeader, audioContentTypeHeader) || event.Content.Type == config.ContentTypeAudio {
		return config.AudioType
	}
	if strings.Contains(typeHeader, articleContentTypeHeader) {
		return config.ArticleType
	}
	if strings.Contains(typeHeader, liveEventContentTypeHeader) {
		return config.LiveEventType
	}

	contentMetadata := h.mapper.Config.ContentMetadataMap
	for _, identifier := range event.Content.Identifiers {
		for _, t := range contentMetadata {
			if t.Authority != "" && strings.Contains(identifier.Authority, t.Authority) {
				return t.ContentType
			}
		}
	}
	originHeader := msg.Headers[originHeader]
	for _, t := range contentMetadata {
		if t.Origin != "" && strings.Contains(originHeader, t.Origin) {
			return t.ContentType
		}
	}
	return ""
}

func isAllowedType(s string) bool {
	// Empty type added for older content. Placeholders - which are subject of exclusion - have type Content.
	var allowedTypes = [...]string{"Article", "Video", "MediaResource", "Audio", "ContentPackage", ""}
	for _, value := range allowedTypes {
		if value == s {
			return true
		}
	}
	return false
}
