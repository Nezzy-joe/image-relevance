FROM golang:1.26.0-alpine3.23 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/evaluate ./cmd/evaluate
RUN mkdir -p /out/image-store /out/certs /out/tmp \
    && chmod 1777 /out/tmp \
    && cp /etc/ssl/cert.pem /out/certs/ca-certificates.crt

FROM scratch
LABEL org.opencontainers.image.title="image-relevance" \
      org.opencontainers.image.description="AI Image Understanding & Content Matching Engine" \
      org.opencontainers.image.source="https://github.com/Nezzy-joe/image-relevance"
COPY --from=build /out/api /api
COPY --from=build /out/seed /seed
COPY --from=build /out/worker /worker
COPY --from=build /out/evaluate /evaluate
COPY --from=build /out/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build --chown=65532:65532 /out/tmp /tmp
COPY --from=build --chown=65532:65532 /out/image-store /var/lib/image-relevance/images
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/api"]
