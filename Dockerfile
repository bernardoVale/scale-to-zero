FROM golang:1 as builder

WORKDIR /usr/src

ADD . .

RUN CGO_ENABLED=0 go build -o scale

FROM scratch

COPY --from=builder /usr/src/scale /

EXPOSE 8080

ENV DEBUG=true

CMD [ "/scale" ]
