FROM alpine:3.6

RUN apk add -U ca-certificates \
  && update-ca-certificates \
  && rm -rf /var/cache/apk/*

ADD bin/linux/k8s-ec2-srcdst /k8s-ec2-srcdst

CMD ["/k8s-ec2-srcdst"]
