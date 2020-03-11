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

type ContentTypeMap map[string]schema.ContentType
type Map map[string]string

func (c Map) Get(key string) string {
	return c[strings.ToLower(key)]
}

func (c ContentTypeMap) Get(key string) schema.ContentType {
	return c[strings.ToLower(key)]
}

type AppConfig struct {
	Predicates     Map
	ConceptTypes   Map
	Origins        Map
	Authorities    Map
	ContentTypeMap ContentTypeMap
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

	origins := v.Sub("content").GetStringMapString("origin")
	predicates := v.GetStringMapString("predicates")
	concepts := v.GetStringMapString("conceptTypes")
	authorities := v.GetStringMapString("authorities")
	var contentTypeMap ContentTypeMap
	err = v.UnmarshalKey("esContentTypeMap", &contentTypeMap)
	if err != nil {
		return AppConfig{}, fmt.Errorf("unable to unmarshal %w", err)
	}

	return AppConfig{
		Predicates:     predicates,
		ConceptTypes:   concepts,
		Origins:        origins,
		Authorities:    authorities,
		ContentTypeMap: contentTypeMap,
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
