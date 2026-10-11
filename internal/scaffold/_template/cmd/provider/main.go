// package main implements the [[ .ProjectName ]] provider.
package main

import (
	"flag"
	"os"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/openeverest/openeverest/v2/provider-runtime/reconciler"

	"[[ .ModulePath ]]/internal/provider"
)

// main is the entry point for the provider.
func main() {
	var serverPort int
	var metricsBindAddress string
	var admission bool

	flag.IntVar(&serverPort, "server-port", 8082, "The port for the provider HTTP server.")
	flag.StringVar(&metricsBindAddress, "metrics-bind-address", ":8081", "The address the metrics endpoint binds to. Use 0 to disable.")
	flag.BoolVar(&admission, "admission", true, "Serve the Instance admission webhook, with the serving certificate the chart mounts.")
	flag.Parse()

	l := ctrl.Log.WithName("setup")
	ctx := ctrl.SetupSignalHandler()

	p := provider.New()

	opts := []reconciler.ReconcilerOption{
		// Serve the health and readiness probes.
		reconciler.WithServer(reconciler.ServerConfig{Port: serverPort}),
	}

	if metricsBindAddress != "0" {
		opts = append(opts, reconciler.WithMetrics(metricsBindAddress))
	}

	if admission {
		opts = append(opts, reconciler.WithAdmission())
	}

	r, err := reconciler.New(ctx, p, opts...)
	if err != nil {
		l.Error(err, "unable to create reconciler")
		os.Exit(1)
	}

	if err := r.Start(ctx); err != nil {
		l.Error(err, "unable to start reconciler")
		os.Exit(1)
	}
}
