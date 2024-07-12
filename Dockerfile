FROM golang:1.22-bookworm AS build
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY ./ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /fluxcd-suspend-notifier .

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=build fluxcd-suspend-notifier /fluxcd-suspend-notifier
ENTRYPOINT [ "/fluxcd-suspend-notifier" ]
