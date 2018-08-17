FROM alpine

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

ADD cloud-key-rotator /go/bin/cloud-key-rotator

RUN addgroup -S ckrgroup && adduser -S ckruser -G ckrgroup

USER ckruser

RUN mkdir ~/.aws

ENTRYPOINT /go/bin/cloud-key-rotator
