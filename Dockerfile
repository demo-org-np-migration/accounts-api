# syntax=docker/dockerfile:1
FROM golang:1.22 AS build
WORKDIR /src

# cauri-go-kit isn't on the public module proxy checksum db yet.
ENV GOPROXY=direct
ENV GONOSUMDB=github.com/demo-org-np-migration
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN go build -o /out/accounts-api ./cmd/accounts-api

FROM gcr.io/distroless/static-debian12:nonroot
USER nonroot:nonroot
COPY --from=build /out/accounts-api /accounts-api
EXPOSE 8080
ENTRYPOINT ["/accounts-api"]
