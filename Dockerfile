FROM golang:1 as builder

WORKDIR /usr

RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

WORKDIR $GOPATH/src/github.com/bernardoVale/scale-to-zero

ADD Gopkg* ./
RUN dep ensure --vendor-only

ADD . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o /app .
# RUN CGO_ENABLED=0 go build -o scale

FROM scratch

COPY --from=builder /app ./

EXPOSE 8080
ENV DEBUG=true

ENTRYPOINT ["./app"]
