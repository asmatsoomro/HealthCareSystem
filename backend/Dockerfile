# Multi-stage build for HealthCarePortal backend
FROM golang:1.21 as builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/healthcareportal .

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /
COPY --from=builder /bin/healthcareportal /healthcareportal
EXPOSE 8080
USER nonroot:nonroot
ENV ADDR=:8080
ENTRYPOINT ["/healthcareportal"]
