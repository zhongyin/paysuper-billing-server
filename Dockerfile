FROM golang:1.11-alpine AS builder

RUN apk add bash ca-certificates git

WORKDIR /application

ENV GO111MODULE=on

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -o $GOPATH/bin/paysuper_billing_service .

ENV MONGO_HOST = "localhost:3002"
ENV MONGO_DB = "payone"
ENV MONGO_USER = ""
ENV MONGO_PASSWORD = ""
ENV CENTRIFUGO_SECRET = "secret"
ENV CARD_PAY_ORDER_CREATE_URL = "https://sandbox.cardpay.com"

ENTRYPOINT $GOPATH/bin/paysuper_billing_service