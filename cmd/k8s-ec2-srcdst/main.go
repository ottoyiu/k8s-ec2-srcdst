package main // import "github.com/ottoyiu/k8s-ec2-srcdst/cmd/k8s-ec2-srcdst"
import (
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/informers"

	"github.com/ottoyiu/k8s-ec2-srcdst/pkg/signals"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"github.com/ottoyiu/k8s-ec2-srcdst"
	"github.com/ottoyiu/k8s-ec2-srcdst/pkg/common"
	"github.com/ottoyiu/k8s-ec2-srcdst/pkg/controller"
	"k8s.io/client-go/kubernetes"
)

func main() {
	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	numWorkers := flag.Int("numWorkers", 1, "Number of workers to run in the controller")
	version := flag.Bool("version", false, "Prints current k8s-ec2-srcdst version")

	flag.Set("logtostderr", "true")
	flag.Parse()

	if *version {
		fmt.Println(srcdst.Version)
		os.Exit(0)
	}

	// Build the client config - optionally using a provided kubeconfig file.
	config, err := common.GetClientConfig(*kubeconfig)
	if err != nil {
		glog.Fatalf("Failed to load client config: %v", err.Error())
	}

	// Construct the Kubernetes client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create kubernetes client: %v", err.Error())
	}

	glog.Infof("k8s-ec2-srcdst: %v", srcdst.Version)

	awsConfig := &aws.Config{}
	awsSession, err := session.NewSession(awsConfig)
	if err != nil {
		glog.Fatalf("Failed to create AWS session: %v", err.Error())
	}

	ec2Client := ec2.New(awsSession, awsConfig)

	stopCh := signals.SetupSignalHandler()

	nodeInformerFactory := informers.NewSharedInformerFactory(client, time.Second*30)

	srcDstController := controller.NewSrcDstController(client, nodeInformerFactory.Core().V1().Nodes(),
		ec2Client)

	go nodeInformerFactory.Start(stopCh)

	if err = srcDstController.Run(*numWorkers, stopCh); err != nil {
		glog.Fatalf("Error running controller: %v", err.Error())
	}
}
