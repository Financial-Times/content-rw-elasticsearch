package config

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/Financial-Times/content-rw-elasticsearch/pkg/schema"
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

var ProjectRoot = getProjectRoot()

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

func ParseConfig(configFilePath string) (AppConfig, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(joinPath(ProjectRoot, configFilePath))
	if err := v.ReadInConfig(); err != nil {
		return AppConfig{}, err
	}
	origins := v.Sub("content").GetStringMapString("origin")
	predicates := v.GetStringMapString("predicates")
	concepts := v.GetStringMapString("conceptTypes")
	authorities := v.GetStringMapString("authorities")
	var contentTypeMap ContentTypeMap
	err := v.UnmarshalKey("esContentTypeMap", &contentTypeMap)
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

func GetResourceFilePath(resourceFilePath string) string {
	return joinPath(ProjectRoot, resourceFilePath)
}

func getProjectRoot() string {
	currentDir, _ := os.Getwd()
	for !strings.HasSuffix(currentDir, "content-rw-elasticsearch") {
		currentDir = path.Dir(currentDir)
	}
	return currentDir
}

func joinPath(source, target string) string {
	if path.IsAbs(target) {
		return target
	}
	return path.Join(source, target)
}
