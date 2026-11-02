FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY services/go.mod services/go.sum ./
RUN go mod download
COPY services/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./api/cmd

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && addgroup -S app && adduser -S -G app app \
    && mkdir -p /app/logs && chown app:app /app/logs
WORKDIR /app
COPY --from=build /out/api /usr/local/bin/api
COPY services/api/config/config.yaml /app/config/config.yaml
USER app
EXPOSE 8080
ENTRYPOINT ["api"]
CMD ["-config", "/app/config/config.yaml", "-env-file", ""]
