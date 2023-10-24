package node

import (
	"errors"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/gorilla/mux"
	"github.com/rocket-pool/rocketpool-go/node"
	"github.com/rocket-pool/rocketpool-go/rocketpool"

	"github.com/rocket-pool/smartnode/rocketpool/common/server"
	"github.com/rocket-pool/smartnode/rocketpool/common/voting"
	"github.com/rocket-pool/smartnode/shared/types/api"
)

// ===============
// === Factory ===
// ===============

type nodeGetSnapshotVotingPowerContextFactory struct {
	handler *NodeHandler
}

func (f *nodeGetSnapshotVotingPowerContextFactory) Create(vars map[string]string) (*nodeGetSnapshotVotingPowerContext, error) {
	c := &nodeGetSnapshotVotingPowerContext{
		handler: f.handler,
	}
	return c, nil
}

func (f *nodeGetSnapshotVotingPowerContextFactory) RegisterRoute(router *mux.Router) {
	server.RegisterQuerylessRoute[*nodeGetSnapshotVotingPowerContext, api.NodeGetSnapshotVotingPowerData](
		router, "get-snapshot-voting-power", f, f.handler.serviceProvider,
	)
}

// ===============
// === Context ===
// ===============

type nodeGetSnapshotVotingPowerContext struct {
	handler *NodeHandler
	rp      *rocketpool.RocketPool

	node *node.Node
}

func (c *nodeGetSnapshotVotingPowerContext) PrepareData(data *api.NodeGetSnapshotVotingPowerData, opts *bind.TransactOpts) error {
	sp := c.handler.serviceProvider
	cfg := sp.GetConfig()
	nodeAddress, _ := sp.GetWallet().GetAddress()

	// Requirements
	err := errors.Join(
		sp.RequireNodeRegistered(),
		sp.RequireSnapshot(),
	)
	if err != nil {
		return err
	}

	data.VotingPower, err = voting.GetSnapshotVotingPower(cfg, nodeAddress)
	if err != nil {
		return err
	}
	return nil
}