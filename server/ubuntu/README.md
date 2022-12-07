## Pulling from docker

`docker pull juicelabs/server:<release>-<os>`

**Supported OSs**
- ubuntu-18
- ubuntu-20
- ubuntu-22

## Running a Juice container

Juice images require the NVidia runtime as well as a couple of host files to be mounted in the container.

```
1. docker run
3. --gpus all
4. -v /usr/share/vulkan/icd.d/nvidia_icd.json:/usr/share/vulkan/icd.d/nvidia_icd.json
5. -v /usr/share/glvnd/egl_vendor.d/10_nvidia.json:/usr/share/glvnd/egl_vendor.d/10_nvidia.json
6. -it juicelabs/server:<release>-<os>
```

Line 3 tells NVidia runtime to extend the GPUs in to the container.

Lines 4-5 are a bit tricky as I cannot guarantee this is the proper location. However, for Vulkan to work in the container I needed to mount the vulkan json file and the egl json file in the container for the NVidia driver to work properly. Vulkan searches /usr/share/vulkan/icd.d as well as /etc/vulkan/icd.d and others. If /usr/share/vulkan/icd.d does not contain nvidia_icd.json and the driver is properly installed, check /etc/vulkan/icd.d and pass that path through the -v argument instead.

## Juice Options

The main option to change would be the port the Juice server will use. By default, we set this to 43210. The image will expose that port as well by default. However, to change the port, add the `-e PORT=<port>` to the docker run command with the desired port. You will also need to add `-p <port>` to expose that port in the container.

## Juice Log Files

Juice log files are stored in /home/juice/agent/log. Adding a volume at that path would allow you to preserve those log files between container runs.
