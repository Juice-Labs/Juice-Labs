ARG BASE_CONTAINER

FROM golang:latest as build

COPY . /tmp/juice/

RUN cd /tmp/juice && \
    go build -ldflags "-X cmd.internal.build.version=${JUICE_VERSION} -X github.com/Juice-Labs/Juice-Labs/pkg/sentry.SentryDsn=${JUICE_SENTRY_DSN}" ./cmd/controller

FROM ${BASE_CONTAINER}

ARG JUICE_VERSION
ENV JUICE_VERSION=${JUICE_VERSION}
LABEL "juicelabs.co/version"=${JUICE_VERSION}
LABEL maintainer="JUICE TECHNOLOGIES <juicelabs.co>"

ARG PORT=8080
ENV PORT=${PORT}
EXPOSE ${PORT}

WORKDIR /root

COPY --from=build /tmp/juice/controller .

ENTRYPOINT ["/bin/bash", "-c", "./controller --address 0.0.0.0:${PORT} ${@}", "--"]
