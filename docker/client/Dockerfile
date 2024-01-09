ARG BASE_CONTAINER
FROM ${BASE_CONTAINER}

RUN apt-get update && \
    apt-get install -y wget

ARG JUICE_VERSION
ENV JUICE_VERSION=${JUICE_VERSION}
LABEL "juicelabs.co/version"=${JUICE_VERSION}
RUN wget -O ~/JuiceClient.tar.gz https://github.com/Xdevlab/Run/releases/download/${JUICE_VERSION}/JuiceClient-linux.tar.gz

WORKDIR /root

RUN tar -xf JuiceClient.tar.gz && \
    rm -f JuiceClient.tar.gz

ENTRYPOINT ["/bin/bash", "-c", "/root/juicify", "--"]

LABEL maintainer="JUICE TECHNOLOGIES <juicelabs.co>"
