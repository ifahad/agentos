# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /sql-connector ./cmd/sql-connector

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /sql-connector /sql-connector
EXPOSE 8090
ENTRYPOINT ["/sql-connector"]
