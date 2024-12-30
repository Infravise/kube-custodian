package main

import (
	danglingpods "kube-custodian/resources/dangling-pods"
	ephemeralresources "kube-custodian/resources/ephemeral-resources"
	"log"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func setLogLevel() *logrus.Logger {
	log := logrus.New()
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "DEBUG":
		log.SetLevel(logrus.DebugLevel)
		return log
	case "INFO":
		log.SetLevel(logrus.InfoLevel)
		return log
	case "WARN":
		log.SetLevel(logrus.WarnLevel)
		return log
	case "ERROR":
		log.SetLevel(logrus.ErrorLevel)
		return log
	default:
		log.SetLevel(logrus.InfoLevel)
		return log
	}
}

func initializeClient(log *logrus.Logger) (discoveryClient *discovery.DiscoveryClient, dynamicClient *dynamic.DynamicClient, kubeClient *kubernetes.Clientset) {
	// Create the in-cluster config
	log.Infoln("Creating kubernetes client config")
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create client config, %v", err)
	}
	// Create the Discovery Client
	discoveryClient, err = discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create client [Discovery], %v", err)
	}
	// Create the Dynamic Client
	dynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create client [Dynamic], %v", err)
	}
	// Create the Kubernetes Client
	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create client [Kubernetes], %v", err)
	}
	return discoveryClient, dynamicClient, kubeClient
}

func main() {
	// Create tmp/health file for probes
	file, err := os.Create("/tmp/health")
	if err == nil {
		file.WriteString("healthy")
		file.Close()
	} else {
		log.Fatalf("Failed to generate file for health check, %v", err)
	}
	// Initialize logging & log level
	log := setLogLevel()
	// Initialize API clients
	discoveryClient, dynamicClient, kubeClient := initializeClient(log)

	for {
		// Cleanup any dangling pods
		danglingpods.Cleanup(log, kubeClient)
		// Cleanup any expired ephemeral resources
		ephemeralresources.Cleanup(log, discoveryClient, dynamicClient)
		// Sleep 30 seconds then loop again
		time.Sleep(30 * time.Second)
	}
}
