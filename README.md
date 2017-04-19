# secure-environment - A loader for secure environments

Forked from [virtru/secure-environment](https://github.com/virtru/secure-environment).

## Introduction

This tool is intended to be used on the start up of a Docker container to securely fetch and decrypt environment variables stored in S3 and encrypted with a KMS key. The included `secure-entrypoint.sh` script can be used along with the `secure-environment` binary.

### How it works

The `docker-entrypoint.sh` script acts as an entrypoint for the Docker container. The script then calls the `secure-environment` binary to write a sourceable shell script to stdout that contains `export`ed environment variables.

### Setting up the Docker container

To use this with Convox, you need to set the label `convox.environment.secure=true` to true on the services you intend to secure. 

On your Docker container the `secure-entrypoint.sh` in the scripts folder of this repository and the latest Linux binary of the `secure-environment` executable should be copied into your Docker image at the following locations:

```
secure-environment -> /usr/sbin/secure-environment
secure-entrypoint -> /usr/sbin/secure-entrypoint.sh
```

Finally, you need to set the `ENTRYPOINT` on your Dockerfile to this:

```
ENTRYPOINT ["/usr/sbin/secure-entrypoint.sh"]
```

See [https://github.com/virtru/secure-environment](https://github.com/virtru/secure-environment) for example usage.
