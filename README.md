# kubernetes-ec2-srcdst-controller
A Kubernetes Controller that will ensure that Source/Dest Check on the nodes within the cluster that are EC2 instances, are disabled.
This is useful for Calico deployments in AWS where routing within a VPC subnet can be possible without IPIP encapsulation.

## Quick Start
To deploy this controller into your Kubernetes cluster, please make sure your cluster fufills the requirements as listed below.

Then run the following to deploy the stable version of kubernetes-ec2-srcdst-controller:
```
kubectl create -f raw.github.com/FILLTHISOUT
```


## Requirements
kubernetes-ec2-srcdst-controller must have the ability to access the Kubernetes API for a list of nodes and also ability to add an annotation to a node (write access). Please ensure the service account has sufficient access if ran in-cluster. Otherwise, please make sure that the user specified in the kubeconfig has sufficient permissions.

kubernetes-ec2-srcdst-controller also needs the ability to modify the EC2 instance attributes of the nodes running in the Kubernetes cluster. Please make sure to schedule the controller on a node with the IAM policy:
- `ec2:ModifyInstanceAttribute`

If you are running a Kubernetes cluster in AWS created by kops, only the master node(s) have that IAM policy set (`ec2:*`). The example `deploy/controller.yaml` already sets the NodeAffinity to only deploy the controller on one of the master nodes.


## Usage
To deploy this controller into your Kubernetes cluster, please make sure your cluster fufills the requirements as listed above.

First, checkout this repository, and with the working directory set as the root of this repository, run the `kubectl create` command:
```
git clone https://github.com/ottoyiu/kubernetes-ec2-srcdst-controller.git
cd kubernetes-ec2-srcdst-controller
kubectl create -f deploy/controller.yaml
```

```
Usage of ./kubernetes-ec2-srcdst-controller:
  -alsologtostderr
        log to standard error as well as files
  -kubeconfig string
        Path to a kubeconfig file
  -log_backtrace_at value
        when logging hits line file:N, emit a stack trace
  -log_dir string
        If non-empty, write log files in this directory
  -logtostderr
        log to standard error instead of files
  -stderrthreshold value
        logs at or above this threshold go to stderr
  -v value
        log level for V logs
  -vmodule value
        comma-separated list of pattern=N settings for file-filtered logging
```
Specifying the verbosity level of logging to 4 using the `-v` flag will get debug level output.

You only need to specify the location to kubeconfig using the `-kubeconfig` flag if you are running the controller out of the cluster for development and testing purpose.

As well, if you are running this controller outside of the cluster or a node that does not have the proper IAM instance profile, you can specify AWS credentials as environmental variables:

### Environmental Variables
Variable                       | Description
------------------------------ | ----------
`AWS_REGION`                   | Region Name (eg. us-west-2)
`AWS_ACCESS_KEY`               | AWS Access Key (Optional if using IAM instance profiles)
`AWS_SECRET_ACCESS_KEY`        | AWS Secret Access Key (Optional if using IAM instance profiles)


