# Dev container image for [[ .ProjectName ]].
#
# The `dev` stage is used by the Tilt dev workflow (dev/Tiltfile), which builds
# it with bin/ as the context — not the repository root — so the only file here
# is the pre-built binary. Tilt live-updates it in place on rebuild.
FROM alpine AS dev
WORKDIR /home/provider
RUN chown 65534:65534 /home/provider
COPY --chown=65534:65534 ./provider ./provider
USER 65534:65534
ENTRYPOINT ["/home/provider/provider"]
