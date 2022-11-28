package config

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/schema"
	"github.com/spf13/viper"
)

const (
	AppName            = "content-rw-elasticsearch"
	AppDescription     = "Content Read Writer for Elasticsearch"
	AppDefaultLogLevel = "INFO"

	ArticleType   = "article"
	VideoType     = "video"
	BlogType      = "blog"
	AudioType     = "audio"
	LiveEventType = "live-event"

	PACOrigin = "http://cmdb.ft.com/systems/pac"

	ContentTypeAudio = "Audio"
)

type ESContentTypeMetadataMap map[string]schema.ContentType
type Map map[string]string
type ContentMetadataMap map[string]ContentMetadata

type ContentMetadata struct {
	Origin      string
	Authority   string
	ContentType string
}

func (c Map) Get(key string) string {
	return c[strings.ToLower(key)]
}

func (c ESContentTypeMetadataMap) Get(key string) schema.ContentType {
	return c[strings.ToLower(key)]
}

func (c ContentMetadataMap) Get(key string) ContentMetadata {
	return c[strings.ToLower(key)]
}

type AppConfig struct {
	Predicates               Map
	ConceptTypes             Map
	ContentMetadataMap       ContentMetadataMap
	ESContentTypeMetadataMap ESContentTypeMetadataMap
}

func ParseConfig(configFileName string) (AppConfig, error) {
	contents, err := ReadConfigFile(configFileName)
	if err != nil {
		return AppConfig{}, err
	}

	v := viper.New()
	v.SetConfigType("yaml")
	if err = v.ReadConfig(bytes.NewBuffer(contents)); err != nil {
		return AppConfig{}, err
	}

	var contentMetadataMap ContentMetadataMap
	err = v.UnmarshalKey("contentMetadata", &contentMetadataMap)
	if err != nil {
		return AppConfig{}, fmt.Errorf("unable to unmarshal %w", err)
	}

	predicates := v.GetStringMapString("predicates")
	concepts := v.GetStringMapString("conceptTypes")
	var contentTypeMetadataMap ESContentTypeMetadataMap
	err = v.UnmarshalKey("esContentTypeMetadata", &contentTypeMetadataMap)
	if err != nil {
		return AppConfig{}, fmt.Errorf("unable to unmarshal %w", err)
	}

	return AppConfig{
		Predicates:               predicates,
		ConceptTypes:             concepts,
		ContentMetadataMap:       contentMetadataMap,
		ESContentTypeMetadataMap: contentTypeMetadataMap,
	}, nil
}

func ReadConfigFile(fileName string) ([]byte, error) {
	path, err := filepath.Abs(fmt.Sprintf("./configs/%s", fileName))
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	return io.ReadAll(file)
}
