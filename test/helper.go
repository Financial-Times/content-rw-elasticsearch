package test

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/config"
)

func getResourceFilePath(resourceFilePath string) string {
	return joinPath(getProjectRoot(), resourceFilePath)
}

func getProjectRoot() string {
	currentDir, _ := os.Getwd()
	for !strings.HasSuffix(currentDir, config.AppName) {
		currentDir = path.Dir(currentDir)
		if currentDir == "/" {
			break
		}
	}
	return currentDir
}

func joinPath(source, target string) string {
	if path.IsAbs(target) {
		return target
	}
	return path.Join(source, target)
}

func ReadTestResource(testDataFileName string) []byte {
	filePath := getResourceFilePath("test/testdata/" + testDataFileName)
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		e := fmt.Errorf("cannot read test resource '%s': %s", testDataFileName, err)
		log.Fatal(e)
	}
	return content
}
