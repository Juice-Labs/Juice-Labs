FROM ubuntu:jammy

RUN \
    apt-get update && \
    apt-get install -y libvulkan1 libgl1 libglib2.0-0 wget

RUN \
    wget https://github.com/Juice-Labs/Juice-Labs/releases/latest/download/JuiceClient-ubuntu-18.tar.gz && \
    mkdir JuiceClient && \
    tar -xf ../JuiceClient-ubuntu-18.tar.gz -C JuiceClient

RUN \
    apt-get update && \
    apt-get install -y python3 python3-pip && \
    pip3 install torch torchvision torchaudio --extra-index-url https://download.pytorch.org/whl/cu117

WORKDIR /JuiceClient
