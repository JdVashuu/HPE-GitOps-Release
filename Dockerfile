# Stage 1: Build
FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go-backend/go.mod go-backend/go.sum ./
RUN go mod download
COPY go-backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/recipe-api ./cmd/recipe-api

# Stage 2: Run
FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache ca-certificates git
COPY --from=build /app/recipe-api /app/recipe-api
EXPOSE 8081
ENTRYPOINT ["/app/recipe-api"]
