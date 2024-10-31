/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	machineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/kyma-project/template-operator/api/v1alpha1"
	"github.com/kyma-project/template-operator/controllers"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	rateLimiterBurstDefault     = 200
	rateLimiterFrequencyDefault = 30
	failureBaseDelayDefault     = 1 * time.Second
	failureMaxDelayDefault      = 1000 * time.Second
	operatorName                = "template-operator"
	webhookPort                 = 9443
)

type FlagVar struct {
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
	failureBaseDelay     time.Duration
	failureMaxDelay      time.Duration
	rateLimiterFrequency int
	rateLimiterBurst     int
	finalState           string
	finalDeletionState   string
	printVersion         bool
}

func registerSchemes(scheme *machineryruntime.Scheme) {
	machineryutilruntime.Must(clientgoscheme.AddToScheme(scheme))
	machineryutilruntime.Must(controllers.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

//nolint:gochecknoglobals // used to embed static binary version during release builds
var buildVersion = "not_provided"

func main() {
	scheme := machineryruntime.NewScheme()
	setupLog := ctrl.Log.WithName("setup")
	registerSchemes(scheme)

	flagVar := defineFlagVar()
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if flagVar.printVersion {
		msg := fmt.Sprintf("Template Operator version: %s\n", buildVersion)
		_, err := os.Stdout.WriteString(msg)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	rateLimiter := controllers.RateLimiter{
		Burst:           flagVar.rateLimiterBurst,
		Frequency:       flagVar.rateLimiterFrequency,
		BaseDelay:       flagVar.failureBaseDelay,
		FailureMaxDelay: flagVar.failureMaxDelay,
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: flagVar.metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: webhookPort,
		}),
		HealthProbeBindAddress: flagVar.probeAddr,
		LeaderElection:         flagVar.enableLeaderElection,
		LeaderElectionID:       "76223278.kyma-project.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.SampleReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		EventRecorder:      mgr.GetEventRecorderFor(operatorName),
		FinalState:         v1alpha1.State(flagVar.finalState),
		FinalDeletionState: v1alpha1.State(flagVar.finalDeletionState),
	}).SetupWithManager(mgr, rateLimiter); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Sample")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func defineFlagVar() *FlagVar {
	flagVar := new(FlagVar)
	flag.StringVar(&flagVar.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&flagVar.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&flagVar.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&flagVar.rateLimiterBurst, "rate-limiter-burst", rateLimiterBurstDefault,
		"Indicates the burst value for the bucket rate limiter.")
	flag.IntVar(&flagVar.rateLimiterFrequency, "rate-limiter-frequency", rateLimiterFrequencyDefault,
		"Indicates the bucket rate limiter frequency, signifying no. of events per second.")
	flag.DurationVar(&flagVar.failureBaseDelay, "failure-base-delay", failureBaseDelayDefault,
		"Indicates the failure base delay in seconds for rate limiter.")
	flag.DurationVar(&flagVar.failureMaxDelay, "failure-max-delay", failureMaxDelayDefault,
		"Indicates the failure max delay in seconds")
	flag.StringVar(&flagVar.finalState, "final-state", string(v1alpha1.StateReady),
		"Customize final state, to mimic state behaviour like Ready, Warning")
	flag.StringVar(&flagVar.finalDeletionState, "final-deletion-state", string(v1alpha1.StateDeleting),
		"Customize final state when module marked for deletion, to mimic state behaviour like Ready, Warning")
	flag.BoolVar(&flagVar.printVersion, "version", false, "Prints the operator version and exits")
	return flagVar
}
