FROM golang:1.9.2-alpine
RUN apk add --update git
WORKDIR /go/src/cadrspace_bot
COPY . $WORKDIR
RUN go get -u github.com/golang/dep/...
RUN dep ensure
RUN go build

FROM alpine:latest  
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /
COPY --from=0 /go/src/cadrspace_bot/cadrspace_bot .
CMD ["./cadrspace_bot"]  
