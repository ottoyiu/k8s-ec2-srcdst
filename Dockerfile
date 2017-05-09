FROM golang:1.7-alpine

ADD bin/linux/k8s-ec2-srcdst /k8s-ec2-srcdst

CMD ["/k8s-ec2-srcdst"]
