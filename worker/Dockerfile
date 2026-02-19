FROM golang:1.23-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/worker .

FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app

COPY --from=build /out/worker /app/worker

EXPOSE 9090

ENTRYPOINT ["/app/worker"]
