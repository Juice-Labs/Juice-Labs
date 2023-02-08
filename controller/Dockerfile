FROM node:14.16.0

ARG PORT=8080
EXPOSE ${PORT}
ENV PORT=${PORT}

WORKDIR /root/controller

COPY . .
RUN ./compile.sh

ENTRYPOINT ["/bin/bash", "-c", "node dist/bin/main.js --port $PORT ${@}", "--"]

LABEL maintainer="JUICE TECHNOLOGIES <juicelabs.co>"
