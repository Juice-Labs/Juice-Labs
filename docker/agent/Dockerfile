ARG BASE_CONTAINER


FROM golang:latest as build

COPY . /tmp/juice/

RUN cd /tmp/juice && \
    go build ./cmd/agent


# This base container has the video codec sdk stored at /
# The zip file requires a license agreement to be accepted for download
# Retrieved from https://developer.nvidia.com/nvidia-video-codec-sdk
ARG VIDEOSDK_VERSION=12.0.16
FROM 597595123756.dkr.ecr.us-east-2.amazonaws.com/juicelabs/nvidia/video-codec:${VIDEOSDK_VERSION} AS video-codec


FROM ${BASE_CONTAINER}

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y software-properties-common wget unzip pciutils

# Install libstdc++-10
RUN add-apt-repository ppa:ubuntu-toolchain-r/test && \
    apt-get install -y libstdc++-10-dev

# Install opengl support
RUN apt-get install -y libglvnd0 libgl1 libegl1

# Install vulkan support
RUN apt-get install -y libvulkan1

# Largely pulled from https://gitlab.com/nvidia/container-images/video-codec-sdk/-/blob/master/ubuntu18.04/Dockerfile
ARG VIDEOSDK_VERSION

RUN mkdir -p /opt/nvidia/video-sdk
WORKDIR /opt/nvidia/video-sdk
COPY --from=video-codec /Video_Codec_SDK_${VIDEOSDK_VERSION}.zip /opt/nvidia/video-sdk
RUN unzip -j Video_Codec_SDK_${VIDEOSDK_VERSION}.zip \
          Video_Codec_SDK_${VIDEOSDK_VERSION}/Interface/cuviddec.h \
          Video_Codec_SDK_${VIDEOSDK_VERSION}/Interface/nvcuvid.h \
          Video_Codec_SDK_${VIDEOSDK_VERSION}/Interface/nvEncodeAPI.h \
          -d /usr/local/cuda/include && \
    unzip -j Video_Codec_SDK_${VIDEOSDK_VERSION}.zip \
          Video_Codec_SDK_${VIDEOSDK_VERSION}/Lib/linux/stubs/x86_64/libnvcuvid.so \
          Video_Codec_SDK_${VIDEOSDK_VERSION}/Lib/linux/stubs/x86_64/libnvidia-encode.so \
          -d /usr/local/cuda/lib64/stubs && \
    rm -fr Video_Codec_SDK_${VIDEOSDK_VERSION}.zip

# This needs to be fixed. Shouldn't be linking to libnvidia-encode.so.1
RUN mv /usr/local/cuda/lib64/stubs/libnvidia-encode.so /usr/local/cuda/lib64/stubs/libnvidia-encode.so.1 && \
    ln -s -T /usr/local/cuda/lib64/stubs/libnvidia-encode.so.1 /usr/local/cuda/lib64/stubs/libnvidia-encode.so

# Seems the NVidia runtime does not expose these files
# To get around needing to mount the host version, just adding a default version
COPY nvidia_icd.json /usr/share/vulkan/icd.d/nvidia_icd.json
COPY 10_nvidia.json /usr/share/glvnd/egl_vendor.d/10_nvidia.json

ARG VK_API_VERSION=1.2.203
LABEL "juicelabs.co/vulkan/api_version"=${VK_API_VERSION}
RUN sed -e "s/<<VK_API_VERSION>>/${VK_API_VERSION}/" /usr/share/vulkan/icd.d/nvidia_icd.json

# Finally, Juice installation
#ARG JUICE_VERSION
#ENV JUICE_VERSION=${JUICE_VERSION}
#LABEL "juicelabs.co/version"=${JUICE_VERSION}
#RUN wget -O ~/JuiceServer.tar.gz https://github.com/Xdevlab/Run/releases/download/${JUICE_VERSION}/JuiceServer-linux.tar.gz

ARG PORT=43210
ENV PORT=${PORT}
EXPOSE ${PORT}

ENV LD_LIBRARY_PATH=${LD_LIBRARY_PATH}:/usr/local/cuda/lib64:/usr/local/cuda/lib64/stubs:/root
ENV FONTCONFIG_PATH=/etc/fonts

ENV NVIDIA_DISABLE_REQUIRE=1
ENV NVIDIA_DRIVER_CAPABILITIES=compute,graphics,utility,video

WORKDIR /root

COPY --from=build /tmp/juice/agent .

ENTRYPOINT ["/bin/bash", "-c", "./agent --address 0.0.0.0:${PORT}", "--"]

LABEL maintainer="JUICE TECHNOLOGIES <juicelabs.co>"
