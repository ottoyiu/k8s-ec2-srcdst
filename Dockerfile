FROM alpine:3.5

ADD bin/linux/kubernetes-ec2-srcdst-controller /kubernetes-ec2-srcdst-controller

CMD ["/kubernetes-ec2-srcdst-controller"]
