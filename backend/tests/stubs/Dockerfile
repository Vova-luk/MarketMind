FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/stub *.go

FROM alpine:3.22
COPY --from=build /out/stub /usr/local/bin/stub
USER nobody
ENTRYPOINT ["stub"]
