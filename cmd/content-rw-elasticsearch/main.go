package main

import (
	"net/http"
	"os"
	"time"

	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/concept"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/config"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/es"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/health"
	pkghttp "github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/http"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/mapper"
	"github.com/Financial-Times/content-rw-elasticsearch/v4/pkg/message"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v3"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	cli "github.com/jawher/mow.cli"
)

const defaultESTestingEndpoint = "http://localhost:9000"

func main() {
	app := cli.App(config.AppName, config.AppDescription)

	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "content-rw-elasticsearch",
		Desc:   "System Code of the application",
		EnvVar: "APP_SYSTEM_CODE",
	})
	appName := app.String(cli.StringOpt{
		Name:   "app-name",
		Value:  config.AppName,
		Desc:   "Application name",
		EnvVar: "APP_NAME",
	})
	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
		Desc:   "Port to listen on",
		EnvVar: "APP_PORT",
	})
	logLevel := app.String(cli.StringOpt{
		Name:   "logLevel",
		Value:  config.AppDefaultLogLevel,
		Desc:   "Logging level (DEBUG, INFO, WARN, ERROR)",
		EnvVar: "LOG_LEVEL",
	})
	esEndpoint := app.String(cli.StringOpt{
		Name:   "elasticsearch-sapi-endpoint",
		Value:  "http://localhost:9200",
		Desc:   "AES endpoint",
		EnvVar: "ELASTICSEARCH_SAPI_ENDPOINT",
	})
	esRegion := app.String(cli.StringOpt{
		Name:   "elasticsearch-region",
		Value:  "local",
		Desc:   "AES region",
		EnvVar: "ELASTICSEARCH_REGION",
	})
	indexName := app.String(cli.StringOpt{
		Name:   "index-name",
		Value:  "ft",
		Desc:   "The name of the elasticsearch index",
		EnvVar: "ELASTICSEARCH_SAPI_INDEX",
	})
	kafkaAddress := app.String(cli.StringOpt{
		Name:   "kafka-address",
		Value:  "kafka:9092",
		Desc:   "Addresses used by the queue consumer to connect to Kafka",
		EnvVar: "KAFKA_ADDR",
	})
	kafkaConsumerGroup := app.String(cli.StringOpt{
		Name:   "kafka-consumer-group",
		Value:  "content-rw-elasticsearch",
		Desc:   "Group used to read the messages from the queue",
		EnvVar: "KAFKA_CONSUMER_GROUP",
	})
	kafkaTopic := app.String(cli.StringOpt{
		Name:   "kafka-topic",
		Value:  "CombinedPostPublicationEvents",
		Desc:   "The topic to read the messages from",
		EnvVar: "KAFKA_TOPIC",
	})
	kafkaTopicOffsetFetchInterval := app.Int(cli.IntOpt{
		Name:   "kafka-topic-offset-fetch-interval",
		Desc:   "Interval (in minutes) between each offset fetching request",
		EnvVar: "KAFKA_TOPIC_OFFSET_FETCH_INTERVAL",
	})
	kafkaLagTolerance := app.Int(cli.IntOpt{
		Name:   "kafka-topic-lag-tolerance",
		Desc:   "Lag tolerance (in number of messages) used for monitoring the Kafka consumer",
		EnvVar: "KAFKA_TOPIC_LAG_TOLERANCE",
	})
	publicConcordancesEndpoint := app.String(cli.StringOpt{
		Name:   "public-concordances-endpoint",
		Value:  "http://public-concordances-api:8080",
		Desc:   "Endpoint to concord ids with",
		EnvVar: "PUBLIC_CONCORDANCES_ENDPOINT",
	})
	baseAPIUrl := app.String(cli.StringOpt{
		Name:   "base-api-url",
		Value:  "https://api.ft.com/",
		Desc:   "Base API URL",
		EnvVar: "BASE_API_URL",
	})

	log := logger.NewUPPLogger(*appSystemCode, *logLevel)
	log.Info("[Startup] Application is starting")

	app.Action = func() {
		accessConfig := es.AccessConfig{
			AwsCreds: credentials.AnonymousCredentials,
			Endpoint: *esEndpoint,
			Region:   *esRegion,
		}

		if *esEndpoint != defaultESTestingEndpoint {
			log.Info("Trying to get session credentials")
			awsSession, sessionErr := session.NewSession()
			if sessionErr != nil {
				log.WithError(sessionErr).Fatal("Failed to initialize AWS session")
			}
			credValues, err := awsSession.Config.Credentials.Get()
			if err != nil {
				log.WithError(err).Fatal("Failed to obtain AWS credentials values")
			}
			awsCreds := awsSession.Config.Credentials
			log.Infof("Obtaining AWS credentials by using [%s] as provider", credValues.ProviderName)

			accessConfig = es.AccessConfig{
				AwsCreds: awsCreds,
				Endpoint: *esEndpoint,
				Region:   *esRegion,
			}
		}

		httpClient := pkghttp.NewHTTPClient()

		appConfig, err := config.ParseConfig("app.yml")
		if err != nil {
			log.Fatal(err)
		}

		esService := es.NewService(*indexName)
		concordanceAPIService := concept.NewConcordanceAPIService(*publicConcordancesEndpoint, httpClient)
		mapperHandler := mapper.NewMapperHandler(concordanceAPIService, *baseAPIUrl, appConfig, log)

		consumerConfig := kafka.ConsumerConfig{
			BrokersConnectionString: *kafkaAddress,
			ConsumerGroup:           *kafkaConsumerGroup,
			ConnectionRetryInterval: time.Minute,
			OffsetFetchInterval:     time.Duration(*kafkaTopicOffsetFetchInterval) * time.Minute,
			Options:                 kafka.DefaultConsumerOptions(),
		}
		topics := []*kafka.Topic{
			kafka.NewTopic(*kafkaTopic, kafka.WithLagTolerance(int64(*kafkaLagTolerance))),
		}
		messageConsumer := kafka.NewConsumer(consumerConfig, topics, log)

		handler := message.NewMessageHandler(
			esService,
			mapperHandler,
			httpClient,
			messageConsumer,
			es.NewClient,
			log,
		)

		handler.Start(*baseAPIUrl, accessConfig)

		healthService := health.NewHealthService(messageConsumer, esService, httpClient, concordanceAPIService, *appSystemCode, log)

		serveMux := http.NewServeMux()
		serveMux = healthService.AttachHTTPEndpoints(serveMux, *appName, config.AppDescription)
		pkghttp.StartServer(log, serveMux, *port)

		handler.Stop()
	}
	err := app.Run(os.Args)
	if err != nil {
		log.WithError(err).WithTime(time.Now()).Fatal("App could not start")
		return
	}
	log.Info("[Shutdown] Shutdown complete")
}
