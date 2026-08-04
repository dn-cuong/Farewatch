FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -o /out/scanner ./cmd/scanner

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
 && adduser -D -H -u 10001 farewatch
WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/scanner /app/scanner
USER farewatch
EXPOSE 8080
ENTRYPOINT ["/app/api"]
