FROM golang:1.27.4-trixie AS build
WORKDIR /app
COPY go.* ./
RUN go mod download

COPY ./ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /fluxcd-suspend-notifier .

FROM gcr.io/distroless/static-debian13 AS app
WORKDIR /
COPY --from=build /fluxcd-suspend-notifier /fluxcd-suspend-notifier
ENTRYPOINT [ "/fluxcd-suspend-notifier" ]
