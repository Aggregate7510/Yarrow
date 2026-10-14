# Offline smoke-test image: same runtime contract as the real Dockerfile
# (non-root uid 1000, /data volume, env, HEALTHCHECK, entrypoint) but built
# FROM scratch with a pre-compiled static binary, because this environment
# cannot pull base images.
FROM scratch
COPY ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --chown=1000:1000 data /data
COPY --chown=1000:1000 tmp /tmp
COPY yarrow /usr/local/bin/yarrow
USER 1000:1000
ENV YARR_ADDR=0.0.0.0:7070 YARR_DB=/data/storage.db
VOLUME /data
EXPOSE 7070
HEALTHCHECK --interval=2s --timeout=3s --start-period=2s --retries=3 CMD ["/usr/local/bin/yarrow", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/yarrow"]
