# content-rw-elasticsearch

[![Circle CI](https://circleci.com/gh/Financial-Times/content-rw-elasticsearch/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/content-rw-elasticsearch/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/content-rw-elasticsearch)](https://goreportcard.com/report/github.com/Financial-Times/content-rw-elasticsearch) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/content-rw-elasticsearch/badge.svg)](https://coveralls.io/github/Financial-Times/content-rw-elasticsearch)


## Introduction
Indexes V2 content in Elasticsearch for use by SAPI V1

## Installation
Download the source code, dependencies and test dependencies:

        go get -u github.com/kardianos/govendor
        go get -u github.com/Financial-Times/content-rw-elasticsearch
        cd $GOPATH/src/github.com/Financial-Times/content-rw-elasticsearch
        govendor sync
        go build .

## Running locally

1. Run the tests and install the binary:

        govendor sync
        govendor test -v -race
        go install

2. Run the binary (using the `help` flag to see the available optional arguments):

        $GOPATH/bin/content-rw-elasticsearch [--help]

Options:

        --app-system-code="content-rw-elasticsearch"            System Code of the application ($APP_SYSTEM_CODE)
        --app-name="Content RW Elasticsearch"                   Application name ($APP_NAME)
        --port="8080"                                           Port to listen on ($APP_PORT)
        --aws-access-key=""                                     AWS ACCES KEY ($AWS_ACCESS_KEY_ID)
        --aws-secret-access-key=""                              AWS SECRET ACCES KEY ($AWS_SECRET_ACCESS_KEY)
        --elasticsearch-sapi-endpoint="http://localhost:9200"   AES endpoint ($ELASTICSEARCH_SAPI_ENDPOINT)
        --index-name="ft"                                       The name of the elaticsearch index ($ELASTICSEARCH_SAPI_INDEX)
        --kafka-proxy-address="http://localhost:8080"           Addresses used by the queue consumer to connect to the queue ($KAFKA_PROXY_ADDR)
        --kafka-consumer-group="default-consumer-group"         Group used to read the messages from the queue ($KAFKA_CONSUMER_GROUP)
        --kafka-topic="CombinedPostPublicationEvents"           The topic to read the meassages from ($KAFKA_TOPIC)
        --kafka-header="kafka"                                  The header identifying the queue to read the messages from ($KAFKA_HEADER)
        --kafka-concurrent-processing=false                     Whether the consumer uses concurrent processing for the messages ($KAFKA_CONCURRENT_PROCESSING)

3. Test:

There are no service endpoints to test.

## Build and deployment

* Built by Docker Hub on merge to master: [coco/content-rw-elasticsearch](https://hub.docker.com/r/coco/content-rw-elasticsearch/)
* CI provided by CircleCI: [content-rw-elasticsearch](https://circleci.com/gh/Financial-Times/content-rw-elasticsearch)

## Healthchecks
Admin endpoints are:

    `/__gtg`

Returns 503 if any if the checks executed at the /__health endpoint returns false

    `/__health`
    
There are several checks performed:
* Elasticsearch cluster connectivity
* Elasticsearch cluster health
* Elastic schema validation
* Kafka queue topic check


    `/__health-details`
    
Shows ES cluster health details

    `/__build-info` 


## Other information
An example of event structure is here [testdata/exampleEnrichedContentModel.json](testdata/exampleEnrichedContentModel.json)

The reference mappings for Elasticsearch are found here [runtime/referenceSchema.json](runtime/referenceSchema.json)

### Logging

* The application uses [logrus](https://github.com/Sirupsen/logrus); the log file is initialised in [main.go](main.go).
* NOTE: `/__build-info` and `/__gtg` endpoints are not logged as they are called every second from varnish/vulcand and this information is not needed in logs/splunk.
