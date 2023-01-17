package controllers

import (
	"github.com/ipfs-cluster/ipfs-cluster/cmdutils"
	ma "github.com/multiformats/go-multiaddr"
)

func GetDefaultServiceConfig() *cmdutils.ConfigHelper {
	cfgHelper := cmdutils.NewConfigHelper("", "", "crdt", "badger")
	cfgHelper.Manager().Default()
	return cfgHelper
}

func EnableMetrics(serviceConfig *cmdutils.Configs) {
	serviceConfig.Metrics.EnableStats = true
	serviceConfig.Metrics.PrometheusEndpoint, _ = ma.NewMultiaddr("/ip4/0.0.0.0/tcp/8888")
}
