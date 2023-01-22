ARG BASE_CONTAINER
FROM ${BASE_CONTAINER}

ARG JUICE_VERSION
ENV JUICE_VERSION=${JUICE_VERSION}
LABEL "juicelabs.co/version"=${JUICE_VERSION}
RUN wget -O ~/JuiceClient.tar.gz https://github.com/Juice-Labs/Juice-Labs/releases/download/${JUICE_VERSION}/JuiceClient-linux.tar.gz

WORKDIR /root

RUN tar -xf JuiceClient.tar.gz && \
    rm -f JuiceClient.tar.gz

ENTRYPOINT ["/bin/bash", "-c", "/root/juicify", "--"]

LABEL maintainer="JUICE TECHNOLOGIES <juicelabs.co>"
