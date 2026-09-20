# Build the binary against the module cache, then ship it alone: the runtime
# image holds no toolchain, no shell, and no source. The SQLite driver is pure
# Go, so CGO stays off and the static base image is enough.
FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cookd-api ./cmd/cookd-api
# An empty data directory to carry over with the right owner: a fresh named
# volume copies ownership from the image, and the nonroot user must be able to
# write the database there.
RUN mkdir /out/data

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/cookd-api /usr/local/bin/cookd-api
COPY --from=build --chown=nonroot:nonroot /out/data /data

USER nonroot:nonroot
EXPOSE 50001
ENTRYPOINT ["/usr/local/bin/cookd-api"]
