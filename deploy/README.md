# Deploy
There are two folders: pre_k8s-1.6 and post_k8s-1.6. Please use the mainfest files that correspond to the version of your Kubernetes cluster.
This is due to the new scheduler of Kubernetes 1.6 ignoring tolerations/taints specified as alpha annotations instead of actual fields. In the future, it will also include a service account for use with RBAC.


# Quick Start
Note that you likely want to change AWS_REGION.
```
AWS_REGION=us-east-1
SSL_CERT_PATH="/etc/ssl/certs/ca-certificates.crt" # (/etc/ssl/certs for GCE instead)

wget -O kubernetes_ec2_srcdst_controller.yaml https://raw.githubusercontent.com/ottoyiu/kubernetes-ec2-srcdst-controller/master/deploy/k8s-1.5_and_prior/v0.0.1.yaml

sed -i -e "s@{{AWS_REGION}}@${AWS_REGION}@g" kubernetes_ec2_srcdst_controller.yaml
sed -i -e "s@{{SSL_CERT_PATH}}@${SSL_CERT_PATH}@g" kubernetes_ec2_srcdst_controller.yaml

kubectl apply -f kubernetes_ec2_srcdst_controller.yaml
```
