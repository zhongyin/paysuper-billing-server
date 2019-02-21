Billing service
=====

[![Build Status](https://travis-ci.org/paysuper/paysuper-billing-server.svg?branch=master)](https://travis-ci.org/paysuper/paysuper-billing-server) 
[![codecov](https://codecov.io/gh/paysuper/paysuper-billing-server/branch/master/graph/badge.svg)](https://codecov.io/gh/paysuper/paysuper-billing-server)

This service contain all business logic for payment processing

## Environment variables:

| Name                                 | Required | Default               | Description                                                                                                                  |
|:-------------------------------------|:--------:|:----------------------|:-----------------------------------------------------------------------------------------------------------------------------|
| MONGO_HOST                           | true     | -                     | MongoDB host including port if this needed                                                                                   |
| MONGO_DB                             | true     | -                     | MongoDB database name                                                                                                        |
| MONGO_USER                           | -        | ""                    | MongoDB user for access to database                                                                                          |
| MONGO_PASSWORD                       | -        | ""                    | MongoBD password for access to database                                                                                      |
| PSP_ACCOUNTING_CURRENCY              | -        | EUR                   | PaySuper accounting currency                                                                                                 |
| METRICS_PORT                         | -        | 8086                  | Http server port for health and metrics request                                                                              |
| ENVIRONMENT                          | -        | dev                   | Application environment. For testing and develop use dev environment. For production this variable must be set to prod value |
| CENTRIFUGO_SECRET                    | true     | -                     | Centrifugo secret key                                                                                                        |
| BROKER_ADDRESS                       | -        | amqp://127.0.0.1:5672 | RabbitMQ url address                                                                                                         |
| CARD_PAY_API_URL                     | true     | -                     | CardPay API url to process payments, more in [documentation](https://integration.cardpay.com/v3/)                            | 
| CACHE_CURRENCY_TIMEOUT               | -        | 15552000              | Timeout in seconds to refresh currencies list cache                                                                          |
| CACHE_PROJECT_TIMEOUT                | -        | 10800                 | Timeout in seconds to refresh projects list cache                                                                            |
| CACHE_CURRENCY_RATE_TIMEOUT          | -        | 86400                 | Timeout in seconds to refresh currencies rates cache                                                                         |
| CACHE_VAT_TIMEOUT                    | -        | 2592000               | Timeout in seconds to refresh VAT list cache                                                                                 |
| CACHE_PAYMENT_METHOD_TIMEOUT         | -        | 2592000               | Timeout in seconds to refresh payment methods list cache                                                                     |
| CACHE_COMMISSION_TIMEOUT             | -        | 86400                 | Timeout in seconds to refresh commissions list cache                                                                         |
| CACHE_PROJECT_PAYMENT_METHOD_TIMEOUT | -        | 86400                 | Timeout in seconds to refresh cache of payment methods linked to projects                                                    |

## Docker Deployment

```bash
docker build -f Dockerfile -t paysuper_billing_service .
docker run -d -e "MONGO_HOST=127.0.0.1:27017" -e "MONGO_DB="paysuper" ... e="CACHE_PROJECT_PAYMENT_METHOD_TIMEOUT=600" paysuper_billing_service
```