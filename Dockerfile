FROM golang:1.25-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY . .
RUN go mod download
WORKDIR /src/cmd
RUN go build -o /app/audit_service .

RUN mkdir /app/configs
RUN cp ../configs/app.env /app/configs

FROM golang:1.25-alpine AS bin-unix
COPY --from=build /app /sparksai-audit-service
ENV AGILEAGENT_SERVER_HOMEDIR="/sparksai-audit-service"
ENTRYPOINT ["/sparksai-audit-service/audit_service"]

