package test

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/Financial-Times/content-rw-elasticsearch/pkg/config"
)

func ReadTestResource(testDataFileName string) []byte {
	filePath := config.GetResourceFilePath("test/data/" + testDataFileName)
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		e := fmt.Errorf("cannot read test resource '%s': %s", testDataFileName, err)
		log.Fatal(e)
	}
	return content
}
