package main // import "github.com/ottoyiu/k8s-ec2-srcdst/cmd/k8s-ec2-srcdst"
import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	srcdst "github.com/ottoyiu/k8s-ec2-srcdst"
	"github.com/ottoyiu/k8s-ec2-srcdst/pkg/common"
	"github.com/ottoyiu/k8s-ec2-srcdst/pkg/controller"
	"k8s.io/client-go/kubernetes"
)

func handleSignals(term func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	signal.Stop(c)
	term()
}

func main() {
	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	version := flag.Bool("version", false, "Prints current k8s-ec2-srcdst version")

	flag.Set("logtostderr", "true")
	flag.Parse()

	if *version {
		fmt.Println(srcdst.Version)
		os.Exit(0)
	}

	glog.Infof("k8s-ec2-srcdst: %v", srcdst.Version)

	// Build the client config - optionally using a provided kubeconfig file.
	config, err := common.GetClientConfig(*kubeconfig)
	if err != nil {
		glog.Fatalf("Failed to load client config: %v", err)
	}

	// Construct the Kubernetes client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	awsSession, err := session.NewSession(&aws.Config{})
	if err != nil {
		glog.Fatalf("Failed to create an AWS API client session: %v", err)
	}
	ec2Client := ec2.New(awsSession)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go handleSignals(cancel)
	stop := ctx.Done()

	ctlr := controller.NewSrcDstController(client, ec2Client)
	ctlr.RunUntil(stop)
}
