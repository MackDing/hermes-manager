# syntax=docker/dockerfile:1.7
# Multi-arch runtime image. Binaries are pre-built by the release workflow
# and copied into build/${TARGETOS}/${TARGETARCH}/hermesmanager.
FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETOS
ARG TARGETARCH

COPY build/${TARGETOS}/${TARGETARCH}/hermesmanager /usr/local/bin/hermesmanager

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/hermesmanager"]
