# k8s-ec2-srcdst (formerly as kubernetes-ec2-srcdst-controller)
[![Build Status](https://travis-ci.org/ottoyiu/k8s-ec2-srcdst.svg?branch=master)](https://travis-ci.org/ottoyiu/k8s-ec2-srcdst) [![Go Report Card](https://goreportcard.com/badge/github.com/ottoyiu/k8s-ec2-srcdst)](https://goreportcard.com/report/github.com/ottoyiu/k8s-ec2-srcdst)

A Kubernetes Controller that will ensure that Source/Dest Check on the nodes within the cluster that are EC2 instances, are disabled.
This is useful for Calico deployments in AWS where routing within a VPC subnet can be possible without IPIP encapsulation.

## Quick Start
To deploy this controller into your Kubernetes cluster, please make sure your cluster fufills the requirements as listed below.
Then go to `deploy/README.md` for a quick start guide on how to deploy this to your Kubernetes cluster.


## Requirements
k8s-ec2-srcdst must have the ability to access the Kubernetes API for a list of nodes and also ability to add an annotation to a node (write access). Please ensure the service account has sufficient access if ran in-cluster. Otherwise, please make sure that the user specified in the kubeconfig has sufficient permissions.

k8s-ec2-srcdst also needs the ability to modify the EC2 instance attributes of the nodes running in the Kubernetes cluster. Please make sure to schedule the controller on a node with the IAM policy:
- `ec2:ModifyInstanceAttribute`

If you are running a Kubernetes cluster in AWS created by kops, only the master node(s) have that IAM policy set (`ec2:*`). The deployment mainfest files  (`deploy/*/*.yaml`) already sets the NodeAffinity and Tolerations to only deploy the controller on one of the master nodes.


## Usage
```
Usage of ./bin/linux/k8s-ec2-srcdst:
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
  -version
        Prints current k8s-ec2-srcdst version
  -vmodule value
        comma-separated list of pattern=N settings for file-filtered logging
```
Specifying the verbosity level of logging to 4 using the `-v` flag will get debug level output.

You only need to specify the location to kubeconfig using the `-kubeconfig` flag if you are running the controller out of the cluster for development and testing purpose.

The AWS Region must be set as an environmental variable. As well, if you are running this controller outside of the cluster or a node that does not have the proper IAM instance profile, you will need to specify AWS credentials as environmental variables:

### Environmental Variables
Variable                       | Description
------------------------------ | ----------
`AWS_REGION`                   | Region Name (eg. us-west-2) - *required*
`AWS_ACCESS_KEY`               | AWS Access Key (Optional if using IAM instance profiles)
`AWS_SECRET_ACCESS_KEY`        | AWS Secret Access Key (Optional if using IAM instance profiles)


