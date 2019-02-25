FROM golang:1.11-alpine AS builder

RUN apk add bash ca-certificates git

WORKDIR /application

ENV GO111MODULE=on

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -o ./bin/paysuper_billing_service .

FROM alpine:3.9

WORKDIR /application

COPY --from=builder /application /application
ENTRYPOINT /application/bin/paysuper_billing_service