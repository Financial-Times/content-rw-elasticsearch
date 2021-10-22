package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/concept"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/config"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/es"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/health"
	pkghttp "github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/http"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/mapper"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/pkg/message"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/upp-go-sdk/pkg/api"
	"github.com/Financial-Times/upp-go-sdk/pkg/internalcontent"
	cli "github.com/jawher/mow.cli"
)

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
	accessKey := app.String(cli.StringOpt{
		Name:   "aws-access-key",
		Desc:   "AWS ACCESS KEY",
		EnvVar: "AWS_ACCESS_KEY_ID",
	})
	secretKey := app.String(cli.StringOpt{
		Name:   "aws-secret-access-key",
		Desc:   "AWS SECRET ACCES KEY",
		EnvVar: "AWS_SECRET_ACCESS_KEY",
	})
	esEndpoint := app.String(cli.StringOpt{
		Name:   "elasticsearch-sapi-endpoint",
		Value:  "http://localhost:9200",
		Desc:   "AES endpoint",
		EnvVar: "ELASTICSEARCH_SAPI_ENDPOINT",
	})
	indexName := app.String(cli.StringOpt{
		Name:   "index-name",
		Value:  "ft",
		Desc:   "The name of the elaticsearch index",
		EnvVar: "ELASTICSEARCH_SAPI_INDEX",
	})
	kafkaAddress := app.String(cli.StringOpt{
		Name:   "kafka-address",
		Value:  "http://kafka:9092",
		Desc:   "Addresses used by the queue consumer to connect to the queue",
		EnvVar: "KAFKA_ADDR",
	})
	kafkaConsumerGroup := app.String(cli.StringOpt{
		Name:   "kafka-consumer-group",
		Value:  "default-consumer-group",
		Desc:   "Group used to read the messages from the queue",
		EnvVar: "KAFKA_CONSUMER_GROUP",
	})
	kafkaTopic := app.String(cli.StringOpt{
		Name:   "kafka-topic",
		Value:  "CombinedPostPublicationEvents",
		Desc:   "The topic to read the messages from",
		EnvVar: "KAFKA_TOPIC",
	})
	//kafkaHeader := app.String(cli.StringOpt{
	//	Name:   "kafka-header",
	//	Value:  "kafka",
	//	Desc:   "The header identifying the queue to read the messages from",
	//	EnvVar: "KAFKA_HEADER",
	//})
	//kafkaConcurrentProcessing := app.Bool(cli.BoolOpt{
	//	Name:   "kafka-concurrent-processing",
	//	Value:  false,
	//	Desc:   "Whether the consumer uses concurrent processing for the messages",
	//	EnvVar: "KAFKA_CONCURRENT_PROCESSING",
	//})
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

	internalContentAPIURL := app.String(cli.StringOpt{
		Name:   "internal-content-api-url",
		Value:  "http://internal-content-api:8080",
		Desc:   "URL of the API uses to retrieve lists data from",
		EnvVar: "INTERNAL_CONTENT_API_URL",
	})

	apiBasicAuthUsername := app.String(cli.StringOpt{
		Name:   "api-basic-auth-user",
		Value:  "",
		Desc:   "API Basic Auth username",
		EnvVar: "API_BASIC_USER",
	})

	apiBasicAuthPassword := app.String(cli.StringOpt{
		Name:   "api-basic-auth-pass",
		Value:  "",
		Desc:   "API Basic Auth password",
		EnvVar: "API_BASIC_PASS",
	})

	//queueConfig := consumer.QueueConfig{
	//	Addrs:                []string{*kafkaProxyAddress},
	//	Group:                *kafkaConsumerGroup,
	//	Topic:                *kafkaTopic,
	//	Queue:                *kafkaHeader,
	//	ConcurrentProcessing: *kafkaConcurrentProcessing,
	//}

	log := logger.NewUPPLogger(*appSystemCode, *logLevel)
	log.Info("[Startup] Application is starting")

	app.Action = func() {
		time.Sleep(5 * time.Second)

		accessConfig := es.AccessConfig{
			AccessKey: *accessKey,
			SecretKey: *secretKey,
			Endpoint:  *esEndpoint,
		}

		httpClient := pkghttp.NewHTTPClient()

		appConfig, err := config.ParseConfig("app.yml")
		if err != nil {
			log.Fatal(err)
		}

		esService := es.NewService(*indexName)

		concordanceAPIService := concept.NewConcordanceAPIService(*publicConcordancesEndpoint, httpClient)

		// initialize apiClient
		internalAPIConfig := api.NewConfig(*internalContentAPIURL, *apiBasicAuthUsername, *apiBasicAuthPassword)
		internalContentAPIClient := api.NewClient(*internalAPIConfig, httpClient)
		internalContentClient := internalcontent.NewContentClient(internalContentAPIClient, internalcontent.URLInternalContent)

		mapperHandler := mapper.NewMapperHandler(
			concordanceAPIService,
			*baseAPIUrl,
			appConfig,
			log,
			internalContentClient,
		)

		esClient, err := es.NewClient(accessConfig, httpClient, log)

		if err != nil {
			log.WithError(err).Error("failed to create Elasticsearch client")
		}

		esService.SetClient(esClient)

		handler := message.NewMessageHandler(
			esService,
			mapperHandler,
			httpClient,
			log,
		)

		consumerConfig := kafka.ConsumerConfig{
			BrokersConnectionString: *kafkaAddress,
			ConsumerGroup:           *kafkaConsumerGroup,
			Topics:                  []string{*kafkaTopic},
			Options:                 kafka.DefaultConsumerOptions(),
		}
		kafkaConsumer := kafka.NewPerseverantConsumer(consumerConfig, log, time.Minute)

		go func() {
			kafkaConsumer.StartListening(handler.HandleMessage)
		}()

		healthService := health.NewHealthService(kafkaConsumer, esService, httpClient, concordanceAPIService, *appSystemCode, log)
		serveMux := http.NewServeMux()
		serveMux = healthService.AttachHTTPEndpoints(serveMux, *appName, config.AppDescription)

		go func() {
			pkghttp.StartServer(log, serveMux, *port)
		}()

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		err = kafkaConsumer.Close()
		if err != nil {
			log.WithError(err).Error("Kafka consumer failed to close")
		}
	}
	err := app.Run(os.Args)
	if err != nil {
		log.WithError(err).WithTime(time.Now()).Fatal("App could not start")
		return
	}
	log.Info("[Shutdown] Shutdown complete")
}
