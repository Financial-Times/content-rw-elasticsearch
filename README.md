# content-rw-elasticsearch

[![Circle CI](https://circleci.com/gh/Financial-Times/content-rw-elasticsearch/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/content-rw-elasticsearch/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/content-rw-elasticsearch)](https://goreportcard.com/report/github.com/Financial-Times/content-rw-elasticsearch) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/content-rw-elasticsearch/badge.svg)](https://coveralls.io/github/Financial-Times/content-rw-elasticsearch)

## Introduction

Indexes V2 content in Elasticsearch for use by SAPI V1

## Project Local Execution

```sh
make all
```

to run tests and a clean build of the project.


### Docker Compose

`docker-compose` is used to provide application external components:

* Elasticsearch

and to start the application itself.

**Step 1.** Build application docker image

```sh
export GITHUB_USERNAME="<username>"
export GITHUB_TOKEN="<token>"

docker-compose build --no-cache app
```

**Step 2.** Run Elasticsearch, Zookeeper & Kafka

```sh
docker-compose up -d es zookeeper kafka
```

**Step 3.** Create Elasticsearch index mapping

```sh
cd <project_home>
curl -X PUT http://localhost:9200/ft/ -d @configs/referenceSchema.json
```

**Step 4.** Run application

```sh
docker-compose up -d app
```

**Step 5.** Check health endpoint in browser at `http://localhost:8080/__health`

### Application CLI

**Step 1.** Build project and run tests

```sh
make all
```

or just build project

```sh
make build-readonly
```

**Step 2.** Run the binary (using the `--help` flag to see the available optional arguments):

```sh
<project_home>/content-rw-elasticsearch [--help]
```

```txt
Options:                                    
      --app-system-code                     System Code of the application (env $APP_SYSTEM_CODE) (default "content-rw-elasticsearch")
      --app-name                            Application name (env $APP_NAME) (default "content-rw-elasticsearch")
      --port                                Port to listen on (env $APP_PORT) (default "8080")
      --logLevel                            Logging level (DEBUG, INFO, WARN, ERROR) (env $LOG_LEVEL) (default "INFO")
      --elasticsearch-sapi-endpoint         AES endpoint (env $ELASTICSEARCH_SAPI_ENDPOINT) (default "http://localhost:9200")
      --elasticsearch-region                AES region (env $ELASTICSEARCH_REGION) (default "local")
      --index-name                          The name of the elasticsearch index (env $ELASTICSEARCH_SAPI_INDEX) (default "ft")
      --kafka-address                       Addresses used by the queue consumer to connect to Kafka (env $KAFKA_ADDR) (default "kafka:9092")
      --kafka-consumer-group                Group used to read the messages from the queue (env $KAFKA_CONSUMER_GROUP) (default "content-rw-elasticsearch")
      --kafka-topic                         The topic to read the messages from (env $KAFKA_TOPIC) (default "CombinedPostPublicationEvents")
      --kafka-topic-offset-fetch-interval   Interval (in minutes) between each offset fetching request (env $KAFKA_TOPIC_OFFSET_FETCH_INTERVAL) (default 0)
      --kafka-topic-lag-tolerance           Lag tolerance (in number of messages) used for monitoring the Kafka consumer (env $KAFKA_TOPIC_LAG_TOLERANCE) (default 0)
      --public-concordances-endpoint        Endpoint to concord ids with (env $PUBLIC_CONCORDANCES_ENDPOINT) (default "http://public-concordances-api:8080")
      --base-api-url                        Base API URL (env $BASE_API_URL) (default "https://api.ft.com/")
```

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

An example of event structure is here [testdata/exampleEnrichedContentModel.json](test/testdata/exampleEnrichedContentModel.json)

The reference mappings for Elasticsearch are found here [configs/referenceSchema.json](configs/referenceSchema.json)
