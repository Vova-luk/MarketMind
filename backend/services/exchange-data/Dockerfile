FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY services/go.mod services/go.sum ./
RUN go mod download
COPY services/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/exchange-data ./exchange-data/cmd/exchange-data

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && addgroup -S app && adduser -S -G app app
COPY --from=build /out/exchange-data /usr/local/bin/exchange-data
COPY services/exchange-data/config/config.yaml /app/config/config.yaml
USER app
EXPOSE 9090
ENTRYPOINT ["exchange-data"]
CMD ["-config", "/app/config/config.yaml"]
