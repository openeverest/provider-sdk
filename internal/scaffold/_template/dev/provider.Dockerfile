# Dev container image for [[ .ProviderName ]].
#
# The `dev` stage is used by the Tilt dev workflow (dev/Tiltfile). It expects a
# pre-built `bin/provider` binary in the build context and supports live
# updates via Tilt's restart_process wrapper.
FROM alpine AS dev
WORKDIR /home/provider
RUN chown 65534:65534 /home/provider
COPY --chown=65534:65534 ./bin/provider ./provider
USER 65534:65534
ENTRYPOINT ["/home/provider/provider"]
