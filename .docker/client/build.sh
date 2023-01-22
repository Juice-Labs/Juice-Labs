docker build --pull --rm --build-arg BASE_CONTAINER=ubuntu:20.04 --build-arg JUICE_VERSION=$1 -f "./Dockerfile" -t juicelabs/client:$1 "."
