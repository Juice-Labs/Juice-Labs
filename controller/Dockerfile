FROM node:16-buster

WORKDIR /app
RUN apt-get update && apt-get upgrade -y

COPY . .
RUN npm install

EXPOSE 8080

ENTRYPOINT ["node", "index.js"]