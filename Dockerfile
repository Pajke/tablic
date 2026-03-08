FROM node:20-alpine AS client
WORKDIR /client
COPY client/package*.json ./
RUN npm ci
COPY client/ .
RUN npm run build

FROM golang:1.25-alpine AS builder
WORKDIR /app/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
COPY --from=client /client/dist ./static/
RUN go build -o /tablic .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /tablic .
EXPOSE 3579
CMD ["./tablic"]
