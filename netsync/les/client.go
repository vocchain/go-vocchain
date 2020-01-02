// Copyright 2016 The go-voc-core Authors
// This file is part of the go-voc-core library.
//
// The go-voc-core library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-voc-core library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-voc-core library. If not, see <http://www.gnu.org/licenses/>.

// Package les implements the Light voc-core Subprotocol.
package les

import (
	"fmt"

	"github.com/voc-core/go-voc-core/accounts"
	"github.com/voc-core/go-voc-core/accounts/abi/bind"
	"github.com/voc-core/go-voc-core/common"
	"github.com/voc-core/go-voc-core/common/hexutil"
	"github.com/voc-core/go-voc-core/common/mclock"
	"github.com/voc-core/go-voc-core/consensus"
	"github.com/voc-core/go-voc-core/core"
	"github.com/voc-core/go-voc-core/core/bloombits"
	"github.com/voc-core/go-voc-core/core/rawdb"
	"github.com/voc-core/go-voc-core/core/types"
	"github.com/voc-core/go-voc-core/eth"
	"github.com/voc-core/go-voc-core/eth/downloader"
	"github.com/voc-core/go-voc-core/eth/filters"
	"github.com/voc-core/go-voc-core/eth/gasprice"
	"github.com/voc-core/go-voc-core/event"
	"github.com/voc-core/go-voc-core/internal/ethapi"
	"github.com/voc-core/go-voc-core/light"
	"github.com/voc-core/go-voc-core/log"
	"github.com/voc-core/go-voc-core/node"
	"github.com/voc-core/go-voc-core/p2p"
	"github.com/voc-core/go-voc-core/p2p/enode"
	"github.com/voc-core/go-voc-core/params"
	"github.com/voc-core/go-voc-core/rpc"
)

type Lightvoc-core struct {
	lesCommons

	reqDist    *requestDistributor
	retriever  *retrieveManager
	odr        *LesOdr
	relay      *lesTxRelay
	handler    *clientHandler
	txPool     *light.TxPool
	blockchain *light.LightChain
	serverPool *serverPool

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	ApiBackend     *LesApiBackend
	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager
	netRPCService  *ethapi.PublicNetAPI
}

func New(ctx *node.ServiceContext, config *eth.Config) (*Lightvoc-core, error) {
	chainDb, err := ctx.OpenDatabase("lightchaindata", config.DatabaseCache, config.DatabaseHandles, "eth/db/chaindata/")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis,
		config.OverrideIstanbul, config.OverrideMuirGlacier)
	if _, isCompat := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !isCompat {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	peers := newPeerSet()
	leth := &Lightvoc-core{
		lesCommons: lesCommons{
			genesis:     genesisHash,
			config:      config,
			chainConfig: chainConfig,
			iConfig:     light.DefaultClientIndexerConfig,
			chainDb:     chainDb,
			peers:       peers,
			closeCh:     make(chan struct{}),
		},
		eventMux:       ctx.EventMux,
		reqDist:        newRequestDistributor(peers, &mclock.System{}),
		accountManager: ctx.AccountManager,
		engine:         eth.CreateConsensusEngine(ctx, chainConfig, &config.Ethash, nil, false, chainDb),
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   eth.NewBloomIndexer(chainDb, params.BloomBitsBlocksClient, params.HelperTrieConfirmations),
		serverPool:     newServerPool(chainDb, config.UltraLightServers),
	}
	leth.retriever = newRetrieveManager(peers, leth.reqDist, leth.serverPool)
	leth.relay = newLesTxRelay(peers, leth.retriever)

	leth.odr = NewLesOdr(chainDb, light.DefaultClientIndexerConfig, leth.retriever)
	leth.chtIndexer = light.NewChtIndexer(chainDb, leth.odr, params.CHTFrequency, params.HelperTrieConfirmations)
	leth.bloomTrieIndexer = light.NewBloomTrieIndexer(chainDb, leth.odr, params.BloomBitsBlocksClient, params.BloomTrieFrequency)
	leth.odr.SetIndexers(leth.chtIndexer, leth.bloomTrieIndexer, leth.bloomIndexer)

	checkpoint := config.Checkpoint
	if checkpoint == nil {
		checkpoint = params.TrustedCheckpoints[genesisHash]
	}
	// Note: NewLightChain adds the trusted checkpoint so it needs an ODR with
	// indexers already set but not started yet
	if leth.blockchain, err = light.NewLightChain(leth.odr, leth.chainConfig, leth.engine, checkpoint); err != nil {
		return nil, err
	}
	leth.chainReader = leth.blockchain
	leth.txPool = light.NewTxPool(leth.chainConfig, leth.blockchain, leth.relay)

	// Set up checkpoint oracle.
	oracle := config.CheckpointOracle
	if oracle == nil {
		oracle = params.CheckpointOracles[genesisHash]
	}
	leth.oracle = newCheckpointOracle(oracle, leth.localCheckpoint)

	// Note: AddChildIndexer starts the update process for the child
	leth.bloomIndexer.AddChildIndexer(leth.bloomTrieIndexer)
	leth.chtIndexer.Start(leth.blockchain)
	leth.bloomIndexer.Start(leth.blockchain)

	leth.handler = newClientHandler(config.UltraLightServers, config.UltraLightFraction, checkpoint, leth)
	if leth.handler.ulc != nil {
		log.Warn("Ultra light client is enabled", "trustedNodes", len(leth.handler.ulc.keys), "minTrustedFraction", leth.handler.ulc.fraction)
		leth.blockchain.DisableCheckFreq()
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		leth.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}

	leth.ApiBackend = &LesApiBackend{ctx.ExtRPCEnabled(), leth, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.Miner.GasPrice
	}
	leth.ApiBackend.gpo = gasprice.NewOracle(leth.ApiBackend, gpoParams)

	return leth, nil
}

type LightDummyAPI struct{}

// Etherbase is the address that mining rewards will be send to
func (s *LightDummyAPI) Etherbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("mining is not supported in light mode")
}

// Coinbase is the address that mining rewards will be send to (alias for Etherbase)
func (s *LightDummyAPI) Coinbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("mining is not supported in light mode")
}

// Hashrate returns the POW hashrate
func (s *LightDummyAPI) Hashrate() hexutil.Uint {
	return 0
}

// Mining returns an indication if this node is currently mining.
func (s *LightDummyAPI) Mining() bool {
	return false
}

// APIs returns the collection of RPC services the voc-core package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Lightvoc-core) APIs() []rpc.API {
	apis := ethapi.GetAPIs(s.ApiBackend)
	apis = append(apis, s.engine.APIs(s.BlockChain().HeaderChain())...)
	return append(apis, []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   &LightDummyAPI{},
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.handler.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, true),
			Public:    true,
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		}, {
			Namespace: "les",
			Version:   "1.0",
			Service:   NewPrivateLightAPI(&s.lesCommons),
			Public:    false,
		},
	}...)
}

func (s *Lightvoc-core) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Lightvoc-core) BlockChain() *light.LightChain      { return s.blockchain }
func (s *Lightvoc-core) TxPool() *light.TxPool              { return s.txPool }
func (s *Lightvoc-core) Engine() consensus.Engine           { return s.engine }
func (s *Lightvoc-core) LesVersion() int                    { return int(ClientProtocolVersions[0]) }
func (s *Lightvoc-core) Downloader() *downloader.Downloader { return s.handler.downloader }
func (s *Lightvoc-core) EventMux() *event.TypeMux           { return s.eventMux }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Lightvoc-core) Protocols() []p2p.Protocol {
	return s.makeProtocols(ClientProtocolVersions, s.handler.runPeer, func(id enode.ID) interface{} {
		if p := s.peers.Peer(peerIdToString(id)); p != nil {
			return p.Info()
		}
		return nil
	})
}

// Start implements node.Service, starting all internal goroutines needed by the
// light voc-core protocol implementation.
func (s *Lightvoc-core) Start(srvr *p2p.Server) error {
	log.Warn("Light client mode is an experimental feature")

	// Start bloom request workers.
	s.wg.Add(bloomServiceThreads)
	s.startBloomHandlers(params.BloomBitsBlocksClient)

	s.netRPCService = ethapi.NewPublicNetAPI(srvr, s.config.NetworkId)

	// clients are searching for the first advertised protocol in the list
	protocolVersion := AdvertiseProtocolVersions[0]
	s.serverPool.start(srvr, lesTopic(s.blockchain.Genesis().Hash(), protocolVersion))
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// voc-core protocol.
func (s *Lightvoc-core) Stop() error {
	close(s.closeCh)
	s.peers.Close()
	s.reqDist.close()
	s.odr.Stop()
	s.relay.Stop()
	s.bloomIndexer.Close()
	s.chtIndexer.Close()
	s.blockchain.Stop()
	s.handler.stop()
	s.txPool.Stop()
	s.engine.Close()
	s.eventMux.Stop()
	s.serverPool.stop()
	s.chainDb.Close()
	s.wg.Wait()
	log.Info("Light voc-core stopped")
	return nil
}

// SetClient sets the rpc client and binds the registrar contract.
func (s *Lightvoc-core) SetContractBackend(backend bind.ContractBackend) {
	if s.oracle == nil {
		return
	}
	s.oracle.start(backend)
}
