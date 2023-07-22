docker build --pull --rm --build-arg BASE_CONTAINER=ubuntu:22.04 -f Dockerfile -t juice-labs/controller:%1 "../.."
