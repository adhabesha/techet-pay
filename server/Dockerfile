# Build stage: static Go binary, no external dependencies
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /techet . && mkdir /data

# Run stage: tiny image
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /techet /techet
COPY --from=build --chown=65532:65532 /data /data
# Local data file (used when no remote store is configured). Mount /data as a volume on a real server.
ENV TECHET_DATA=/data/techet-data.json
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/techet"]
