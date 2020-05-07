FROM golang:1.13 as build 
WORKDIR /workdir
COPY . .
RUN ["go", "build", "cmd/frontend/main.go"]

ENTRYPOINT ["go", "run", "cmd/frontend/main.go"]

EXPOSE 80
