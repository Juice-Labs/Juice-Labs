#!/bin/bash
docker build --pull --rm --build-arg BASE_CONTAINER=nvidia/cuda:12.2.0-runtime-ubuntu22.04 -f Dockerfile -t juice-labs/agent:$1 "../.."
