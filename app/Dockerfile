FROM golang

ENV GO111MODULE=on

WORKDIR /bin

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server

ENTRYPOINT ["./server"]
EXPOSE 80