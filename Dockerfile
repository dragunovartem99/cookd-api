# The binary is compiled in CI (see .github/workflows/deploy.yaml) and shipped
# alongside this file: the SQLite driver is a huge pure-Go package, and building it on
# the small VPS took minutes every deploy. The runtime image holds no toolchain, no
# shell, and no source — just the binary. Locally, `make build` produces bin/cookd-api.

# An empty data directory with the right owner: a fresh named volume copies ownership
# from the image, and the nonroot user must be able to write the database there.
FROM alpine AS data
RUN mkdir /data

FROM gcr.io/distroless/static-debian12:nonroot

COPY --chown=nonroot:nonroot bin/cookd-api /usr/local/bin/cookd-api
COPY --from=data --chown=nonroot:nonroot /data /data

USER nonroot:nonroot
EXPOSE 50001
ENTRYPOINT ["/usr/local/bin/cookd-api"]
