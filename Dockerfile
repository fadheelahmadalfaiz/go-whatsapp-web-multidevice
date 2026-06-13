FROM golang:1.25-alpine3.23 AS builder
RUN apk add --no-cache gcc musl-dev gcompat
WORKDIR /whatsapp

COPY ./src/go.mod ./src/go.sum ./
RUN go mod download

COPY ./src .
RUN go build -ldflags="-w -s" -o /app/whatsapp

FROM alpine:3.23
RUN apk add --no-cache ffmpeg libwebp-tools poppler-utils tzdata su-exec

ARG APP_UID=20001
ARG APP_GID=20000
RUN addgroup -g "${APP_GID}" gowa && \
	adduser -D -u "${APP_UID}" -G gowa -h /app gowauser

ENV TZ=UTC
WORKDIR /app

COPY --from=builder /app/whatsapp /app/whatsapp
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh && chown -R gowauser:gowa /app

USER root
ENTRYPOINT ["/entrypoint.sh"]
CMD [ "rest" ]
