docker build --pull --rm --build-arg BASE_CONTAINER=nvidia/cuda:11.8.0-cudnn8-runtime-ubuntu20.04 --build-arg JUICE_VERSION=%1 -f "./Dockerfile" -t juicelabs/server:%1 "."
