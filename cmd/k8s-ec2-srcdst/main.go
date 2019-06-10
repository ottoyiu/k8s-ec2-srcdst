package main // import "github.com/ottoyiu/k8s-ec2-srcdst/cmd/k8s-ec2-srcdst"
import (
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	srcdst "github.com/ottoyiu/k8s-ec2-srcdst"
	"github.com/ottoyiu/k8s-ec2-srcdst/pkg/common"
	"github.com/ottoyiu/k8s-ec2-srcdst/pkg/controller"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func main() {
	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	version := flag.Bool("version", false, "Prints current k8s-ec2-srcdst version")
	patchnode := flag.Bool("patchnode", false, "By default k8s-ec2-srcdst updates the node opbject, this flag turns on patching instead. This can be useful for when AWS api rate limiting is an issue.")
	verbose := flag.Bool("v", false, "Be verbose")

	flag.Set("logtostderr", "true")
	flag.Parse()

	var my_opts = common.K8sEc2SrcdstOpts{
		Patchnode: *patchnode,
		Verbose:   *verbose,
	}

	if *verbose && *patchnode {
		glog.Infof("Patch mode activated")
	}

	if *version {
		fmt.Println(srcdst.Version)
		os.Exit(0)
	}

	// Build the client config - optionally using a provided kubeconfig file.
	config, err := common.GetClientConfig(*kubeconfig)
	if err != nil {
		glog.Fatalf("Failed to load client config: %v", err)
	}

	// Construct the Kubernetes client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create kubernetes client: %v", err)
	}

	glog.Infof("k8s-ec2-srcdst: %v", srcdst.Version)

	awsSession := session.New()
	awsConfig := &aws.Config{}
	ec2Client := ec2.New(awsSession, awsConfig)

	controller.NewSrcDstController(client, ec2Client, &my_opts).Controller.Run(wait.NeverStop)
}
