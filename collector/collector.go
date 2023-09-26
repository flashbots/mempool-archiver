// Package collector contains the mempool collector service
package collector

import (
	"go.uber.org/zap"
)

type CollectorOpts struct {
	Log          *zap.SugaredLogger
	UID          string
	Nodes        []string
	OutDir       string
	CheckNodeURI string

	BloxrouteAuthToken string
	EdenAuthToken      string
	ChainboundAPIKey   string
}

// Start kicks off all the service components in the background
func Start(opts *CollectorOpts) {
	processor := NewTxProcessor(TxProcessorOpts{
		Log:          opts.Log,
		OutDir:       opts.OutDir,
		UID:          opts.UID,
		CheckNodeURI: opts.CheckNodeURI,
	})
	go processor.Start()

	for _, node := range opts.Nodes {
		conn := NewNodeConnection(opts.Log, node, processor.txC)
		conn.StartInBackground()
	}

	if opts.BloxrouteAuthToken != "" {
		blxOpts := BlxNodeOpts{ //nolint:exhaustruct
			Log:        opts.Log,
			AuthHeader: opts.BloxrouteAuthToken,
			URL:        blxDefaultURL, // URL is taken from ENV vars
		}

		// start Websocket or gRPC subscription depending on URL
		blxConn := NewBlxNodeConnection(blxOpts, processor.txC)
		go blxConn.Start()
	}

	if opts.EdenAuthToken != "" {
		blxOpts := BlxNodeOpts{ //nolint:exhaustruct
			Log:        opts.Log,
			AuthHeader: opts.EdenAuthToken,
		}
		blxConn := NewBlxNodeConnection(blxOpts, processor.txC)
		go blxConn.Start()
	}

	if opts.ChainboundAPIKey != "" {
		opts := ChainboundNodeOpts{ //nolint:exhaustruct
			Log:    opts.Log,
			APIKey: opts.ChainboundAPIKey,
		}
		chainboundConn := NewChainboundNodeConnection(opts, processor.txC)
		go chainboundConn.Start()
	}
}
