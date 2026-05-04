// Package main implements a Composition Function.
package main

import (
	"path/filepath"

	"github.com/alecthomas/kong"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/function-sdk-go"
	"github.com/crossplane/function-sdk-go/resource/composed"
	loggingv1beta1 "github.com/kube-logging/logging-operator/pkg/sdk/logging/api/v1beta1"
)

// CLI of this Function.
type CLI struct {
	Debug        bool `short:"d" help:"Emit debug logs in addition to info logs."`
	OutOfCluster bool `help:"Running outside of a Kubernetes cluster."`

	Network     string `help:"Network on which to listen for gRPC connections." default:"tcp"`
	Address     string `help:"Address at which to listen for gRPC connections." default:":9443"`
	TLSCertsDir string `help:"Directory containing server certs (tls.key, tls.crt) and the CA used to verify client certificates (ca.crt)" env:"TLS_SERVER_CERTS_DIR"`
	Insecure    bool   `help:"Run without mTLS credentials. If you supply this flag --tls-server-certs-dir will be ignored."`
}

// Run this Function.
func (c *CLI) Run() error {
	log, err := function.NewLogger(c.Debug)
	if err != nil {
		return err
	}

	// Register logging-operator types with the composed scheme once at
	// startup. The scheme is a process-global, so per-request registration
	// would race on its internal map (concurrent map writes panic).
	if err := loggingv1beta1.AddToScheme(composed.Scheme); err != nil {
		return errors.Wrap(err, "cannot register logging-operator types with composed scheme")
	}

	var config *rest.Config
	if c.OutOfCluster {
		config, err = OutOfClusterConfig()
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	return function.Serve(&Function{log: log, cs: clientset},
		function.Listen(c.Network, c.Address),
		function.MTLSCertificates(c.TLSCertsDir),
		function.Insecure(c.Insecure))
}

func main() {
	ctx := kong.Parse(&CLI{}, kong.Description("A Crossplane Composition Function."))
	ctx.FatalIfErrorf(ctx.Run())
}

func OutOfClusterConfig() (*rest.Config, error) {
	home := homedir.HomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	return config, nil
}
