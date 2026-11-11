# Build the web app, embed it into one Go binary, run that binary.

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server

FROM alpine:3.22
RUN adduser -D -u 10001 app && mkdir /data && chown app /data
USER app
COPY --from=server /out/server /usr/local/bin/server
# ADMIN_USERNAME and ADMIN_PASSWORD must be passed at run time.
ENV DB_PATH=/data/badminton.db ADDR=:8080
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["server"]
