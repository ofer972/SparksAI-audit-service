FROM golang:1.25-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY . .
RUN go mod download
WORKDIR /src/cmd
RUN go build -o /app/audit_service .

RUN mkdir /app/configs
# Copy config file (template if real one doesn't exist, for Railway compatibility)
RUN if [ -f ../configs/app.env ]; then cp ../configs/app.env /app/configs; else cp ../configs/app.env.template /app/configs/app.env; fi

FROM golang:1.25-alpine AS bin-unix
COPY --from=build /app /sparksai-audit-service
ENV AGILEAGENT_SERVER_HOMEDIR="/sparksai-audit-service"
ENTRYPOINT ["/sparksai-audit-service/audit_service"]

