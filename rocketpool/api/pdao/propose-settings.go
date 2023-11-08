package pdao

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rocket-pool/rocketpool-go/node"
	"github.com/rocket-pool/rocketpool-go/settings/protocol"
	"github.com/rocket-pool/smartnode/shared/services"
	"github.com/rocket-pool/smartnode/shared/types/api"
	cliutils "github.com/rocket-pool/smartnode/shared/utils/cli"
	"github.com/rocket-pool/smartnode/shared/utils/eth1"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

func canProposeSetting(c *cli.Context, settingName string, value string) (*api.CanProposePDAOSettingResponse, error) {

	// Get services
	if err := services.RequireNodeWallet(c); err != nil {
		return nil, err
	}
	if err := services.RequireRocketStorage(c); err != nil {
		return nil, err
	}
	cfg, err := services.GetConfig(c)
	if err != nil {
		return nil, err
	}
	w, err := services.GetWallet(c)
	if err != nil {
		return nil, err
	}
	rp, err := services.GetRocketPool(c)
	if err != nil {
		return nil, err
	}
	bc, err := services.GetBeaconClient(c)
	if err != nil {
		return nil, err
	}

	// Response
	response := api.CanProposePDAOSettingResponse{}

	// Get node account
	nodeAccount, err := w.GetNodeAccount()
	if err != nil {
		return nil, err
	}

	// Sync
	var stakedRpl *big.Int
	var lockedRpl *big.Int
	var proposalBond *big.Int
	var wg errgroup.Group

	// Get the node's RPL stake
	wg.Go(func() error {
		var err error
		stakedRpl, err = node.GetNodeRPLStake(rp, nodeAccount.Address, nil)
		return err
	})

	// Get the node's locked RPL
	wg.Go(func() error {
		var err error
		lockedRpl, err = node.GetNodeRPLLocked(rp, nodeAccount.Address, nil)
		return err
	})

	// Get the node's RPL stake
	wg.Go(func() error {
		var err error
		proposalBond, err = protocol.GetProposalBond(rp, nil)
		return err
	})

	// Wait for data
	if err := wg.Wait(); err != nil {
		return nil, err
	}

	response.StakedRpl = stakedRpl
	response.LockedRpl = lockedRpl
	response.ProposalBond = proposalBond

	freeRpl := big.NewInt(0).Sub(stakedRpl, lockedRpl)
	response.InsufficientRpl = (freeRpl.Cmp(proposalBond) < 0)

	// Get the latest finalized block number and corresponding pollard
	blockNumber, pollard, encodedPollard, err := createPollard(rp, cfg, bc)
	if err != nil {
		return nil, fmt.Errorf("error creating pollard: %w", err)
	}
	response.BlockNumber = blockNumber
	response.Pollard = encodedPollard

	// Get the account transactor
	opts, err := w.GetNodeAccountTransactor()
	if err != nil {
		return nil, err
	}

	// Estimate the gas
	valueName := "value"
	switch settingName {
	// CreateLotEnabled
	case protocol.CreateLotEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeCreateLotEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing CreateLotEnabled: %w", err)
		}

	// BidOnLotEnabled
	case protocol.BidOnLotEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeBidOnLotEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing BidOnLotEnabled: %w", err)
		}

	// LotMinimumEthValue
	case protocol.LotMinimumEthValueSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeLotMinimumEthValueGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing LotMinimumEthValue: %w", err)
		}

	// LotMaximumEthValue
	case protocol.LotMaximumEthValueSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeLotMaximumEthValueGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing LotMaximumEthValue: %w", err)
		}

	// LotDuration
	case protocol.LotDurationSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeLotDurationGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing LotDuration: %w", err)
		}

	// LotStartingPriceRatio
	case protocol.LotStartingPriceRatioSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeLotStartingPriceRatioGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing LotStartingPriceRatio: %w", err)
		}

	// LotReservePriceRatio
	case protocol.LotReservePriceRatioSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeLotReservePriceRatioGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing LotReservePriceRatio: %w", err)
		}

	// DepositEnabled
	case protocol.DepositEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeDepositEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing DepositEnabled: %w", err)
		}

	// AssignDepositsEnabled
	case protocol.AssignDepositsEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeAssignDepositsEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing AssignDepositsEnabled: %w", err)
		}

	// MinimumDeposit
	case protocol.MinimumDepositSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMinimumDepositGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MinimumDeposit: %w", err)
		}

	// MaximumDepositPoolSize
	case protocol.MaximumDepositPoolSizeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMaximumDepositPoolSizeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MaximumDepositPoolSize: %w", err)
		}

	// MaximumDepositAssignments
	case protocol.MaximumDepositAssignmentsSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMaximumDepositAssignmentsGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MaximumDepositAssignments: %w", err)
		}

	// MaximumSocializedDepositAssignments
	case protocol.MaximumSocializedDepositAssignmentsSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMaximumSocializedDepositAssignmentsGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MaximumSocializedDepositAssignments: %w", err)
		}

	// DepositFee
	case protocol.DepositFeeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeDepositFeeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing DepositFee: %w", err)
		}

	// MinipoolSubmitWithdrawableEnabled
	case protocol.MinipoolSubmitWithdrawableEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMinipoolSubmitWithdrawableEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MinipoolSubmitWithdrawableEnabled: %w", err)
		}

	// MinipoolLaunchTimeout
	case protocol.MinipoolLaunchTimeoutSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMinipoolLaunchTimeoutGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MinipoolLaunchTimeout: %w", err)
		}

	// BondReductionEnabled
	case protocol.BondReductionEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeBondReductionEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing BondReductionEnabled: %w", err)
		}

	// MaximumMinipoolCount
	case protocol.MaximumMinipoolCountSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMaximumMinipoolCountGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MaximumMinipoolCount: %w", err)
		}

	// MinipoolUserDistributeWindowStart
	case protocol.MinipoolUserDistributeWindowStartSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMinipoolUserDistributeWindowStartGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MinipoolUserDistributeWindowStart: %w", err)
		}

	// MinipoolUserDistributeWindowLength
	case protocol.MinipoolUserDistributeWindowLengthSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMinipoolUserDistributeWindowLengthGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MinipoolUserDistributeWindowLength: %w", err)
		}

	// NodeConsensusThreshold
	case protocol.NodeConsensusThresholdSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeNodeConsensusThresholdGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing NodeConsensusThreshold: %w", err)
		}

	// SubmitBalancesEnabled
	case protocol.SubmitBalancesEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeSubmitBalancesEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing SubmitBalancesEnabled: %w", err)
		}

	// SubmitBalancesFrequency
	case protocol.SubmitBalancesFrequencySettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeSubmitBalancesFrequencyGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing SubmitBalancesFrequency: %w", err)
		}

	// SubmitPricesEnabled
	case protocol.SubmitPricesEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeSubmitPricesEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing SubmitPricesEnabled: %w", err)
		}

	// SubmitPricesFrequency
	case protocol.SubmitPricesFrequencySettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeSubmitPricesFrequencyGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing SubmitPricesEpochs: %w", err)
		}

	// MinimumNodeFee
	case protocol.MinimumNodeFeeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMinimumNodeFeeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MinimumNodeFee: %w", err)
		}

	// TargetNodeFee
	case protocol.TargetNodeFeeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeTargetNodeFeeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing TargetNodeFee: %w", err)
		}

	// MaximumNodeFee
	case protocol.MaximumNodeFeeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMaximumNodeFeeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MaximumNodeFee: %w", err)
		}

	// NodeFeeDemandRange
	case protocol.NodeFeeDemandRangeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeNodeFeeDemandRangeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing NodeFeeDemandRange: %w", err)
		}

	// TargetRethCollateralRate
	case protocol.TargetRethCollateralRateSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeTargetRethCollateralRateGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing TargetRethCollateralRate: %w", err)
		}

	// NetworkPenaltyThreshold
	case protocol.NetworkPenaltyThresholdSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeNetworkPenaltyThresholdGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing NetworkPenaltyThreshold: %w", err)
		}

	// NetworkPenaltyPerRate
	case protocol.NetworkPenaltyPerRateSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeNetworkPenaltyPerRateGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing NetworkPenaltyPerRate: %w", err)
		}

	// SubmitRewardsEnabled
	case protocol.SubmitRewardsEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeSubmitRewardsEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing SubmitRewardsEnabled: %w", err)
		}

	// NodeRegistrationEnabled
	case protocol.NodeRegistrationEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeNodeRegistrationEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing NodeRegistrationEnabled: %w", err)
		}

	// SmoothingPoolRegistrationEnabled
	case protocol.SmoothingPoolRegistrationEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeSmoothingPoolRegistrationEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing SmoothingPoolRegistrationEnabled: %w", err)
		}

	// NodeDepositEnabled
	case protocol.NodeDepositEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeNodeDepositEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing NodeDepositEnabled: %w", err)
		}

	// VacantMinipoolsEnabled
	case protocol.VacantMinipoolsEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeVacantMinipoolsEnabledGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing VacantMinipoolsEnabled: %w", err)
		}

	// MinimumPerMinipoolStake
	case protocol.MinimumPerMinipoolStakeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMinimumPerMinipoolStakeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MinimumPerMinipoolStake: %w", err)
		}

	// MaximumPerMinipoolStake
	case protocol.MaximumPerMinipoolStakeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeMaximumPerMinipoolStakeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing MaximumPerMinipoolStake: %w", err)
		}

	// VoteTime
	case protocol.VoteTimeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeVoteTimeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing VoteTime: %w", err)
		}

	// VoteDelayTime
	case protocol.VoteDelayTimeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeVoteDelayTimeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing VoteDelayTime: %w", err)
		}

	// ExecuteTime
	case protocol.ExecuteTimeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeExecuteTimeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing ExecuteTime: %w", err)
		}

	// ProposalBond
	case protocol.ProposalBondSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeProposalBondGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing ProposalBond: %w", err)
		}

	// ChallengeBond
	case protocol.ChallengeBondSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeChallengeBondGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing ChallengeBond: %w", err)
		}

	// ChallengePeriod
	case protocol.ChallengePeriodSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeChallengePeriodGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing ChallengePeriod: %w", err)
		}

	// ProposalQuorum
	case protocol.ProposalQuorumSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeProposalQuorumGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing ProposalQuorum: %w", err)
		}

	// ProposalVetoQuorum
	case protocol.ProposalVetoQuorumSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeProposalVetoQuorumGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing ProposalVetoQuorum: %w", err)
		}

	// ProposalMaxBlockAge
	case protocol.ProposalMaxBlockAgeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeProposalMaxBlockAgeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing ProposalMaxBlockAge: %w", err)
		}

	// RewardsClaimIntervalTime
	case protocol.RewardsClaimIntervalTimeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		response.GasInfo, err = protocol.EstimateProposeRewardsClaimIntervalTimeGas(rp, newValue, blockNumber, pollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error estimating gas for proposing RewardsClaimIntervalTime: %w", err)
		}

	default:
		return nil, fmt.Errorf("[%s] is not a valid PDAO setting name", settingName)
	}

	// Update & return response
	response.CanPropose = !(response.InsufficientRpl)
	return &response, nil

}

func proposeSetting(c *cli.Context, settingName string, value string, blockNumber uint32, pollard string) (*api.ProposePDAOSettingResponse, error) {

	// Get services
	if err := services.RequireNodeWallet(c); err != nil {
		return nil, err
	}
	if err := services.RequireRocketStorage(c); err != nil {
		return nil, err
	}
	w, err := services.GetWallet(c)
	if err != nil {
		return nil, err
	}
	rp, err := services.GetRocketPool(c)
	if err != nil {
		return nil, err
	}

	// Response
	response := api.ProposePDAOSettingResponse{}

	// Decode the pollard
	truePollard, err := decodePollard(pollard)
	if err != nil {
		return nil, fmt.Errorf("error regenerating pollard: %w", err)
	}

	// Get transactor
	opts, err := w.GetNodeAccountTransactor()
	if err != nil {
		return nil, err
	}

	// Override the provided pending TX if requested
	err = eth1.CheckForNonceOverride(c, opts)
	if err != nil {
		return nil, fmt.Errorf("Error checking for nonce override: %w", err)
	}

	// Submit the proposal
	var proposalID uint64
	var hash common.Hash
	valueName := "value"
	switch settingName {
	// CreateLotEnabled
	case protocol.CreateLotEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeCreateLotEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing CreateLotEnabled: %w", err)
		}

	// BidOnLotEnabled
	case protocol.BidOnLotEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeBidOnLotEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing BidOnLotEnabled: %w", err)
		}

	// LotMinimumEthValue
	case protocol.LotMinimumEthValueSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeLotMinimumEthValue(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing LotMinimumEthValue: %w", err)
		}

	// LotMaximumEthValue
	case protocol.LotMaximumEthValueSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeLotMaximumEthValue(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing LotMaximumEthValue: %w", err)
		}

	// LotDuration
	case protocol.LotDurationSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeLotDuration(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing LotDuration: %w", err)
		}

	// LotStartingPriceRatio
	case protocol.LotStartingPriceRatioSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeLotStartingPriceRatio(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing LotStartingPriceRatio: %w", err)
		}

	// LotReservePriceRatio
	case protocol.LotReservePriceRatioSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeLotReservePriceRatio(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing LotReservePriceRatio: %w", err)
		}

	// DepositEnabled
	case protocol.DepositEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeDepositEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing DepositEnabled: %w", err)
		}

	// AssignDepositsEnabled
	case protocol.AssignDepositsEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeAssignDepositsEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing AssignDepositsEnabled: %w", err)
		}

	// MinimumDeposit
	case protocol.MinimumDepositSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMinimumDeposit(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MinimumDeposit: %w", err)
		}

	// MaximumDepositPoolSize
	case protocol.MaximumDepositPoolSizeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMaximumDepositPoolSize(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MaximumDepositPoolSize: %w", err)
		}

	// MaximumDepositAssignments
	case protocol.MaximumDepositAssignmentsSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMaximumDepositAssignments(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MaximumDepositAssignments: %w", err)
		}

	// MaximumSocializedDepositAssignments
	case protocol.MaximumSocializedDepositAssignmentsSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMaximumSocializedDepositAssignments(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MaximumSocializedDepositAssignments: %w", err)
		}

	// DepositFee
	case protocol.DepositFeeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeDepositFee(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing DepositFee: %w", err)
		}

	// MinipoolSubmitWithdrawableEnabled
	case protocol.MinipoolSubmitWithdrawableEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMinipoolSubmitWithdrawableEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MinipoolSubmitWithdrawableEnabled: %w", err)
		}

	// MinipoolLaunchTimeout
	case protocol.MinipoolLaunchTimeoutSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMinipoolLaunchTimeout(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MinipoolLaunchTimeout: %w", err)
		}

	// BondReductionEnabled
	case protocol.BondReductionEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeBondReductionEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing BondReductionEnabled: %w", err)
		}

	// MaximumMinipoolCount
	case protocol.MaximumMinipoolCountSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMaximumMinipoolCount(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MaximumMinipoolCount: %w", err)
		}

	// MinipoolUserDistributeWindowStart
	case protocol.MinipoolUserDistributeWindowStartSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMinipoolUserDistributeWindowStart(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MinipoolUserDistributeWindowStart: %w", err)
		}

	// MinipoolUserDistributeWindowLength
	case protocol.MinipoolUserDistributeWindowLengthSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMinipoolUserDistributeWindowLength(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MinipoolUserDistributeWindowLength: %w", err)
		}

	// NodeConsensusThreshold
	case protocol.NodeConsensusThresholdSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeNodeConsensusThreshold(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing NodeConsensusThreshold: %w", err)
		}

	// SubmitBalancesEnabled
	case protocol.SubmitBalancesEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeSubmitBalancesEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing SubmitBalancesEnabled: %w", err)
		}

	// SubmitBalancesFrequency
	case protocol.SubmitBalancesFrequencySettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeSubmitBalancesFrequency(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing SubmitBalancesFrequency: %w", err)
		}

	// SubmitPricesEnabled
	case protocol.SubmitPricesEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeSubmitPricesEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing SubmitPricesEnabled: %w", err)
		}

	// SubmitPricesFrequency
	case protocol.SubmitPricesFrequencySettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeSubmitPricesFrequency(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing SubmitPricesFrequency: %w", err)
		}

	// MinimumNodeFee
	case protocol.MinimumNodeFeeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMinimumNodeFee(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MinimumNodeFee: %w", err)
		}

	// TargetNodeFee
	case protocol.TargetNodeFeeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeTargetNodeFee(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing TargetNodeFee: %w", err)
		}

	// MaximumNodeFee
	case protocol.MaximumNodeFeeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMaximumNodeFee(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MaximumNodeFee: %w", err)
		}

	// NodeFeeDemandRange
	case protocol.NodeFeeDemandRangeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeNodeFeeDemandRange(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing NodeFeeDemandRange: %w", err)
		}

	// TargetRethCollateralRate
	case protocol.TargetRethCollateralRateSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeTargetRethCollateralRate(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing TargetRethCollateralRate: %w", err)
		}

	// NetworkPenaltyThreshold
	case protocol.NetworkPenaltyThresholdSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeNetworkPenaltyThreshold(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing NetworkPenaltyThreshold: %w", err)
		}

	// NetworkPenaltyPerRate
	case protocol.NetworkPenaltyPerRateSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeNetworkPenaltyPerRate(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing NetworkPenaltyPerRate: %w", err)
		}

	// SubmitRewardsEnabled
	case protocol.SubmitRewardsEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeSubmitRewardsEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing SubmitRewardsEnabled: %w", err)
		}

	// NodeRegistrationEnabled
	case protocol.NodeRegistrationEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeNodeRegistrationEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing NodeRegistrationEnabled: %w", err)
		}

	// SmoothingPoolRegistrationEnabled
	case protocol.SmoothingPoolRegistrationEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeSmoothingPoolRegistrationEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing SmoothingPoolRegistrationEnabled: %w", err)
		}

	// NodeDepositEnabled
	case protocol.NodeDepositEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeNodeDepositEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing NodeDepositEnabled: %w", err)
		}

	// VacantMinipoolsEnabled
	case protocol.VacantMinipoolsEnabledSettingPath:
		newValue, err := cliutils.ValidateBool(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeVacantMinipoolsEnabled(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing VacantMinipoolsEnabled: %w", err)
		}

	// MinimumPerMinipoolStake
	case protocol.MinimumPerMinipoolStakeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMinimumPerMinipoolStake(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MinimumPerMinipoolStake: %w", err)
		}

	// MaximumPerMinipoolStake
	case protocol.MaximumPerMinipoolStakeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeMaximumPerMinipoolStake(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing MaximumPerMinipoolStake: %w", err)
		}

	// VoteTime
	case protocol.VoteTimeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeVoteTime(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing VoteTime: %w", err)
		}

	// VoteDelayTime
	case protocol.VoteDelayTimeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeVoteDelayTime(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing VoteDelayTime: %w", err)
		}

	// ExecuteTime
	case protocol.ExecuteTimeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeExecuteTime(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing ExecuteTime: %w", err)
		}

	// ProposalBond
	case protocol.ProposalBondSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeProposalBond(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing ProposalBond: %w", err)
		}

	// ChallengeBond
	case protocol.ChallengeBondSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeChallengeBond(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing ChallengeBond: %w", err)
		}

	// ChallengePeriod
	case protocol.ChallengePeriodSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeChallengePeriod(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing ChallengePeriod: %w", err)
		}

	// ProposalQuorum
	case protocol.ProposalQuorumSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeProposalQuorum(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing ProposalQuorum: %w", err)
		}

	// ProposalVetoQuorum
	case protocol.ProposalVetoQuorumSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeProposalVetoQuorum(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing ProposalVetoQuorum: %w", err)
		}

	// ProposalMaxBlockAge
	case protocol.ProposalMaxBlockAgeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeProposalMaxBlockAge(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing ProposalMaxBlockAge: %w", err)
		}

	// RewardsClaimIntervalTime
	case protocol.RewardsClaimIntervalTimeSettingPath:
		newValue, err := cliutils.ValidateBigInt(valueName, value)
		if err != nil {
			return nil, err
		}
		proposalID, hash, err = protocol.ProposeRewardsClaimIntervalTime(rp, newValue, blockNumber, truePollard, opts)
		if err != nil {
			return nil, fmt.Errorf("error proposing RewardsClaimIntervalTime: %w", err)
		}

	default:
		return nil, fmt.Errorf("[%s] is not a valid PDAO setting name", settingName)
	}

	response.ProposalId = proposalID
	response.TxHash = hash

	// Return response
	return &response, nil
}