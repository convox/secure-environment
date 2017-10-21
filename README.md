# secure-environment - A loader for secure environments

Forked from [virtru/secure-environment](https://github.com/virtru/secure-environment).

## Introduction

This tool is intended to be used on the start up of a Docker container to securely fetch and decrypt environment variables stored in S3 and encrypted with a KMS key.

### How it works

The `secure-environment exec` command acts as an entrypoint for the Docker container including the decrypted variables in the command environment.

### Setting up the Docker container

To use this with Convox, you need to set the label `convox.environment.secure=true` to true on the services you intend to secure.

On your Docker container the latest Linux binary of the `secure-environment` executable should be copied into your Docker image at the following locations:

```
COPY secure-environment /usr/sbin/secure-environment
```

Finally, you need to set the `ENTRYPOINT` on your Dockerfile to this:

```
ENTRYPOINT ["/usr/sbin/secure-environment", "exec", "--"]
```

See [https://github.com/convox-examples/secure-env-example](https://github.com/convox-examples/secure-env-example) for example usage.
