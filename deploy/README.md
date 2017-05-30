# Deploy
There are two folders: pre_k8s-1.6 and k8s-1.6. Please use the mainfest files that correspond to the version of your Kubernetes cluster.
This is due to the new scheduler of Kubernetes 1.6 ignoring tolerations/taints specified as alpha annotations instead of actual fields, and additional RBAC mainifests (service account, cluster role, cluster role binding).


# Quick Start
Note that you likely want to change AWS_REGION.
```
AWS_REGION=us-east-1
SSL_CERT_PATH="/etc/ssl/certs/ca-certificates.crt"

wget -O k8s-ec2-srcdst.yaml https://raw.githubusercontent.com/ottoyiu/k8s-ec2-srcdst/master/deploy/pre_k8s-1.6/stable.yaml
sed -i -e "s@{{AWS_REGION}}@${AWS_REGION}@g" k8s-ec2-srcdst.yaml
sed -i -e "s@{{SSL_CERT_PATH}}@${SSL_CERT_PATH}@g" k8s-ec2-srcdst.yaml

kubectl apply -f k8s-ec2-srcdst.yaml
```
