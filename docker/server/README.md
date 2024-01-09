## Pulling from docker

The Juice server docker image is available on [Docker Hub](https://hub.docker.com/r/juicelabs/server/tags) and can be pulled with `docker pull juicelabs/server:<release>-<os>` replacing `<release>`with a release timestamp and Git hash from [Juice-Labs/releases](https://github.com/Xdevlab/Run/releases) and `<os>` with `ubuntu-18`, `ubuntu-20`, or `ubuntu-22` as necessary.

Pull the Juice server release `2022-11-22-1843-b1ffa79d` for Ubuntu 22:

~~~
docker pull juicelabs/server:2022-11-22-1843-b1ffa79d-ubuntu-22
~~~

## Running a Juice container

Running the Juice server in a container requires the `--gpus all` option to extend GPUs into the container.

Run the Juice server release `2022-11-22-1843-b1ffa79d` for Ubuntu 22:

~~~
docker run
--gpus all
-it juicelabs/server:2022-11-22-1843-b1ffa79d-ubuntu-22
~~~

## Setting TCP Port

The Juice server accepts connections on TCP port 43210 by default.  To use a different port pass `-e PORT=<port>` and `-p <port>` to `docker run` replacing `<port>` with the port to listen on, e.g. `-e PORT=12345 -p 12345:12345`.  These options configure the Juice server to listen on a different port and expose that port inside the container respectively.

## Juice Log Files

Log files are stored in `/home/admin/agent/log`.  Add a volume at that path to preserve logs between container runs.
