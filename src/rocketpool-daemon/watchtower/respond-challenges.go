package watchtower

import (
	"fmt"

	"github.com/rocket-pool/node-manager-core/eth"
	"github.com/rocket-pool/node-manager-core/node/wallet"
	"github.com/rocket-pool/rocketpool-go/dao/oracle"
	"github.com/rocket-pool/rocketpool-go/rocketpool"

	"github.com/rocket-pool/node-manager-core/utils/log"
	"github.com/rocket-pool/smartnode/rocketpool-daemon/common/gas"
	"github.com/rocket-pool/smartnode/rocketpool-daemon/common/services"
	"github.com/rocket-pool/smartnode/rocketpool-daemon/common/state"
	"github.com/rocket-pool/smartnode/rocketpool-daemon/common/tx"
	"github.com/rocket-pool/smartnode/rocketpool-daemon/watchtower/utils"
	"github.com/rocket-pool/smartnode/shared/config"
)

// Respond to challenges task
type RespondChallenges struct {
	sp  *services.ServiceProvider
	cfg *config.SmartNodeConfig
	w   *wallet.Wallet
	rp  *rocketpool.RocketPool
	log log.ColorLogger
}

// Create respond to challenges task
func NewRespondChallenges(sp *services.ServiceProvider, logger log.ColorLogger, m *state.NetworkStateManager) *RespondChallenges {
	return &RespondChallenges{
		sp:  sp,
		cfg: sp.GetConfig(),
		w:   sp.GetWallet(),
		rp:  sp.GetRocketPool(),
		log: logger,
	}
}

// Respond to challenges
func (t *RespondChallenges) Run() error {
	nodeAddress, _ := t.w.GetAddress()

	// Log
	t.log.Println("Checking for challenges to respond to...")
	member, err := oracle.NewOracleDaoMember(t.rp, nodeAddress)
	if err != nil {
		return fmt.Errorf("error creating Oracle DAO member binding: %w", err)
	}

	// Check for active challenges
	err = t.rp.Query(nil, nil, member.IsChallenged)
	if err != nil {
		return fmt.Errorf("error checking if member is challenged: %w", err)
	}
	if !member.IsChallenged.Get() {
		return nil
	}

	// Log
	t.log.Printlnf("Node %s has an active challenge against it, responding...", nodeAddress.Hex())

	// Get transactor
	opts, err := t.w.GetTransactor()
	if err != nil {
		return err
	}

	// Create an oDAO manager
	odaoMgr, err := oracle.NewOracleDaoManager(t.rp)
	if err != nil {
		return fmt.Errorf("error creating Oracle DAO manager binding: %w", err)
	}

	// Get the tx info
	txInfo, err := odaoMgr.DecideChallenge(nodeAddress, opts)
	if err != nil {
		return fmt.Errorf("error getting DecideChallenge TX info: %w", err)
	}
	if txInfo.SimulationResult.SimulationError != "" {
		return fmt.Errorf("simulating DecideChallenge TX failed: %s", txInfo.SimulationResult.SimulationError)
	}

	// Print the gas info
	maxFee := eth.GweiToWei(utils.GetWatchtowerMaxFee(t.cfg))
	if !gas.PrintAndCheckGasInfo(txInfo.SimulationResult, false, 0, &t.log, maxFee, 0) {
		return nil
	}

	// Set the gas settings
	opts.GasFeeCap = maxFee
	opts.GasTipCap = eth.GweiToWei(utils.GetWatchtowerPrioFee(t.cfg))
	opts.GasLimit = txInfo.SimulationResult.SafeGasLimit

	// Print TX info and wait for it to be included in a block
	err = tx.PrintAndWaitForTransaction(t.cfg, t.rp, &t.log, txInfo, opts)
	if err != nil {
		return err
	}

	// Log & return
	t.log.Printlnf("Successfully responded to challenge against node %s.", nodeAddress.Hex())
	return nil
}
