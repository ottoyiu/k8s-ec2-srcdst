# calico-ec2-srcdst-controller
A Kubernetes Controller that will ensure that Source/Dest Check on the nodes within the cluster that are EC2 instances, are disabled. This allows Calico to function within a VPC subnet without IPIP encapsulation.

