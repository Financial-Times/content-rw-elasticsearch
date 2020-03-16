package config

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/schema"
	// This blank import is required in order to read the embedded config files
	_ "github.com/Financial-Times/content-rw-elasticsearch/v2/statik"
	"github.com/rakyll/statik/fs"
	"github.com/spf13/viper"
)

const (
	AppName            = "content-rw-elasticsearch"
	AppDescription     = "Content Read Writer for Elasticsearch"
	AppDefaultLogLevel = "INFO"

	ArticleType = "article"
	VideoType   = "video"
	BlogType    = "blog"
	AudioType   = "audio"
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
	contents, err := ReadEmbeddedResource(configFileName)
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

func ReadEmbeddedResource(fileName string) ([]byte, error) {
	statikFS, err := fs.New()
	if err != nil {
		return nil, err
	}
	// Access individual files by their paths.
	f, err := statikFS.Open("/" + fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return contents, err
}
