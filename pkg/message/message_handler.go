package message

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/config"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/es"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/mapper"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/schema"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v2"
	transactionid "github.com/Financial-Times/transactionid-utils-go"
)

const (
	syntheticRequestPrefix   = "SYNTHETIC-REQ-MON"
	transactionIDHeader      = "X-Request-Id"
	originHeader             = "Origin-System-Id"
	contentTypeHeader        = "Content-Type"
	audioContentTypeHeader   = "ft-upp-audio"
	articleContentTypeHeader = "ft-upp-article"
)

type ESClient func(config es.AccessConfig, c *http.Client, log *logger.UPPLogger) (es.Client, error)

type Handler struct {
	esService  es.Service
	Mapper     *mapper.Handler
	httpClient *http.Client
	log        *logger.UPPLogger
}

func NewMessageHandler(service es.Service, mapper *mapper.Handler, httpClient *http.Client, logger *logger.UPPLogger) *Handler {
	indexer := &Handler{esService: service, Mapper: mapper, httpClient: httpClient, log: logger}
	return indexer
}

func (h *Handler) HandleMessage(msg kafka.FTMessage) {
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

	conceptType := h.Mapper.Config.ESContentTypeMetadataMap.Get(contentType).Collection
	if combinedPostPublicationEvent.Deleted {
		_, err = h.esService.DeleteData(conceptType, uuid)
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

	payload := h.Mapper.ToIndexModel(combinedPostPublicationEvent, contentType, tid)

	_, err = h.esService.WriteData(conceptType, uuid, payload)
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
	contentMetadata := h.Mapper.Config.ContentMetadataMap
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
