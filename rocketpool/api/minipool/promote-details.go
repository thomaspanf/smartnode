package minipool

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/mux"
	batch "github.com/rocket-pool/batch-query"
	"github.com/rocket-pool/rocketpool-go/dao/oracle"
	"github.com/rocket-pool/rocketpool-go/minipool"
	"github.com/rocket-pool/rocketpool-go/node"
	"github.com/rocket-pool/rocketpool-go/rocketpool"

	"github.com/rocket-pool/smartnode/rocketpool/common/server"
	"github.com/rocket-pool/smartnode/shared/types/api"
)

// ===============
// === Factory ===
// ===============

type minipoolPromoteDetailsContextFactory struct {
	handler *MinipoolHandler
}

func (f *minipoolPromoteDetailsContextFactory) Create(vars map[string]string) (*minipoolPromoteDetailsContext, error) {
	c := &minipoolPromoteDetailsContext{
		handler: f.handler,
	}
	return c, nil
}

func (f *minipoolPromoteDetailsContextFactory) RegisterRoute(router *mux.Router) {
	server.RegisterMinipoolRoute[*minipoolPromoteDetailsContext, api.MinipoolPromoteDetailsData](
		router, "promote/details", f, f.handler.serviceProvider,
	)
}

// ===============
// === Context ===
// ===============

type minipoolPromoteDetailsContext struct {
	handler *MinipoolHandler
	rp      *rocketpool.RocketPool

	oSettings *oracle.OracleDaoSettings
}

func (c *minipoolPromoteDetailsContext) Initialize() error {
	sp := c.handler.serviceProvider
	c.rp = sp.GetRocketPool()

	// Requirements
	err := errors.Join(
		sp.RequireNodeRegistered(),
	)
	if err != nil {
		return err
	}

	// Bindings
	oMgr, err := oracle.NewOracleDaoManager(c.rp)
	if err != nil {
		return fmt.Errorf("error creating oDAO manager binding: %w", err)
	}
	c.oSettings = oMgr.Settings
	if err != nil {
		return fmt.Errorf("error creating oDAO settings binding: %w", err)
	}
	return nil
}

func (c *minipoolPromoteDetailsContext) GetState(node *node.Node, mc *batch.MultiCaller) {
	c.oSettings.Minipool.PromotionScrubPeriod.Get(mc)
}

func (c *minipoolPromoteDetailsContext) CheckState(node *node.Node, response *api.MinipoolPromoteDetailsData) bool {
	return true
}

func (c *minipoolPromoteDetailsContext) GetMinipoolDetails(mc *batch.MultiCaller, mp minipool.IMinipool, index int) {
	mpv3, success := minipool.GetMinipoolAsV3(mp)
	if success {
		mpv3.GetNodeAddress(mc)
		mpv3.GetStatusTime(mc)
		mpv3.GetVacant(mc)
	}
}

func (c *minipoolPromoteDetailsContext) PrepareData(addresses []common.Address, mps []minipool.IMinipool, data *api.MinipoolPromoteDetailsData) error {
	// Get the time of the latest block
	latestEth1Block, err := c.rp.Client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("Can't get the latest block time: %w", err)
	}
	latestBlockTime := time.Unix(int64(latestEth1Block.Time), 0)

	// Get the promotion details
	details := make([]api.MinipoolPromoteDetails, len(addresses))
	for i, mp := range mps {
		mpCommon := mp.GetCommonDetails()
		mpDetails := api.MinipoolPromoteDetails{
			Address:    mpCommon.Address,
			CanPromote: false,
		}

		// Check its eligibility
		mpv3, success := minipool.GetMinipoolAsV3(mps[i])
		if success && mpv3.IsVacant {
			creationTime := mpCommon.StatusTime.Formatted()
			remainingTime := creationTime.Add(c.oSettings.Minipool.ScrubPeriod.Value.Formatted()).Sub(latestBlockTime)
			if remainingTime < 0 {
				mpDetails.CanPromote = true
			}
		}

		details[i] = mpDetails
	}

	data.Details = details
	return nil
}