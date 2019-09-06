package main

import (
	"fmt"
	"log"
	"os"

	"github.com/eclipse/che-plugin-broker/brokers/metadata"
	"github.com/eclipse/che-plugin-broker/cfg"
	"github.com/eclipse/che-plugin-broker/common"
)

func main() {
	log.SetOutput(os.Stdout)

	cfg.Parse()
	cfg.Print()

	broker := metadata.NewBroker(cfg.UseLocalhostInPluginUrls)

	if cfg.SelfSignedCertificateFilePath != "" {
		common.ConfigureCertPool(cfg.SelfSignedCertificateFilePath)
	}

	if !cfg.DisablePushingToEndpoint {
		statusTun := common.ConnectOrFail(cfg.PushStatusesEndpoint, cfg.Token)
		broker.PushEvents(statusTun)
	}

	pluginFQNs, err := cfg.ParsePluginFQNs()
	if err != nil {
		message := fmt.Sprintf("Failed to process plugin fully qualified names from config: %s", err)
		broker.PubFailed(message)
		broker.PubLog(message)
		log.Fatal(err)
	}
	err = broker.Start(pluginFQNs, cfg.RegistryAddress)
	if err != nil {
		log.Fatal(err)
	}
}
