FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test ./... && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -o /telemetry ./cmd/server
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /telemetry /telemetry
EXPOSE 8080
ENTRYPOINT ["/telemetry"]
