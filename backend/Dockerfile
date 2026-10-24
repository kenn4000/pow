FROM golang:1.25-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/prisoners-of-war

FROM alpine:3.22
RUN addgroup -g 10001 app && adduser -D -u 10001 -G app -h /data app
COPY --from=build /out/server /usr/local/bin/server
ENV HOME=/data
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
CMD ["--ui=web", "--listen=0.0.0.0:8080"]
