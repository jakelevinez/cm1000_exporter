FROM golang:alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /cm1000_exporter

EXPOSE 9527

CMD [ "/cm1000_exporter" ]
