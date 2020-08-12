# UPP - Content RW Elasticsearch

Content RW Elasticsearch indexes content in Elasticsearch for use by SAPI v1.

## Code

content-rw-elasticsearch

## Primary URL

https://upp-prod-delivery-glb.upp.ft.com/__content-rw-elasticsearch/

## Service Tier

Platinum

## Lifecycle Stage

Production

## Delivered By

content

## Supported By

content

## Known About By

- dimitar.terziev
- elitsa.pavlova
- kalin.arsov
- hristo.georgiev
- georgi.ivanov
- elina.kaneva
- robert.marinov
- tsvetan.dimitrov

## Host Platform

AWS

## Architecture

This service receives combined messages (content and metadata) from the `CombinedPostPublicationEvents` Kafka queue and indexes them in Elasticsearch for use by SAPI v1.

## Contains Personal Data

No

## Contains Sensitive Data

No

## Dependencies

* search-api
* kafka-proxy
* public-concordances-api
* up-ica

## Failover Architecture Type

ActiveActive

## Failover Process Type

FullyAutomated

## Failback Process Type

FullyAutomated

## Failover Details

The service is deployed in both Delivery clusters. The failover guide for the cluster is located here: https://github.com/Financial-Times/upp-docs/tree/master/failover-guides/delivery-cluster.

## Data Recovery Process Type

NotApplicable

## Data Recovery Details

The service does not store data, so it does not require any data recovery steps.

## Release Process Type

PartiallyAutomated

## Rollback Process Type

Manual

## Release Details

The release is triggered by making a Github release which is then picked up by a Jenkins multibranch pipeline. The Jenkins pipeline should be manually started in order for it to deploy the helm package to the Kubernetes clusters.

## Key Management Process Type

NotApplicable

## Key Management Details

There is no key rotation procedure for this system.

## Monitoring

Service in UPP K8S delivery clusters:

* Delivery-Prod-EU health: https://upp-prod-delivery-eu.upp.ft.com/__health/__pods-health?service-name=content-rw-elasticsearch
* Delivery-Prod-US health: https://upp-prod-delivery-us.upp.ft.com/__health/__pods-health?service-name=content-rw-elasticsearch

## First Line Troubleshooting

[First Line Troubleshooting guide](https://github.com/Financial-Times/upp-docs/tree/master/guides/ops/first-line-troubleshooting)

## Second Line Troubleshooting

Please refer to the GitHub repository README for troubleshooting information.
