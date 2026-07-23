FROM golang:1.26.3-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tren ./cmd/tren

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/tren /usr/local/bin/tren
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["tren", "serve"]
