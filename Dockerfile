FROM alpine
COPY builds/linux/secure-environment /usr/sbin/secure-environment
ENTRYPOINT ["/usr/sbin/secure-environment", "exec", "--"]
CMD env