FROM golang:1.13
COPY . /workdir
WORKDIR /workdir
RUN go build cmd/frontend/main.go
ENTRYPOINT ["go", "run", "cmd/frontend/main.go"]

EXPOSE 80
