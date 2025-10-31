# syntax=docker/dockerfile:1.7

FROM golang:1.25 AS build

WORKDIR /src

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker

FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker

EXPOSE 8080

ENTRYPOINT ["/app/api"]
