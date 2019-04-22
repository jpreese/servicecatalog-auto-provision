FROM golang

ENV GO111MODULE=on

WORKDIR /bin

COPY . .

RUN go get -d -v

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/statefulmeshservice-controller

ENTRYPOINT ["./statefulmeshservice-controller"]
EXPOSE 80
