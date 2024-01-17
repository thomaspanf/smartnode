package odao

import (
	"github.com/urfave/cli/v2"

	"github.com/rocket-pool/rocketpool-go/dao/oracle"
	"github.com/rocket-pool/rocketpool-go/rocketpool"
	"github.com/rocket-pool/smartnode/rocketpool-cli/utils"
	"github.com/rocket-pool/smartnode/shared/utils/input"
)

// Register commands
func RegisterCommands(app *cli.App, name string, aliases []string) {
	// Create the member settings commands
	membersContract := rocketpool.ContractName_RocketDAONodeTrustedSettingsMembers
	memberSettingsCmd := utils.CreateSetterCategory("members", "Member", "m", membersContract)
	memberSettingsCmd.Subcommands = []*cli.Command{
		utils.CreatePercentSetter("quorum", "q", membersContract, oracle.SettingName_Member_Quorum, proposeSetting),
		utils.CreateRplSetter("rpl-bond", "r", membersContract, oracle.SettingName_Member_RplBond, proposeSetting),
		utils.CreateDurationSetter("challenge-cooldown", "cd", membersContract, oracle.SettingName_Member_ChallengeCooldown, proposeSetting),
		utils.CreateDurationSetter("challenge-window", "cw", membersContract, oracle.SettingName_Member_ChallengeWindow, proposeSetting),
		utils.CreateEthSetter("challenge-cost", "cc", membersContract, oracle.SettingName_Member_ChallengeCost, proposeSetting),
	}

	// Create the minipool settings commands
	minipoolContract := rocketpool.ContractName_RocketDAONodeTrustedSettingsMinipool
	minipoolSettingsCmd := utils.CreateSetterCategory("minipool", "Minipool", "n", minipoolContract)
	minipoolSettingsCmd.Subcommands = []*cli.Command{
		utils.CreateDurationSetter("scrub-period", "sp", minipoolContract, oracle.SettingName_Minipool_ScrubPeriod, proposeSetting),
		utils.CreatePercentSetter("scrub-quorum", "sq", minipoolContract, oracle.SettingName_Minipool_ScrubQuorum, proposeSetting),
		utils.CreateDurationSetter("promotion-scrub-period", "psp", minipoolContract, oracle.SettingName_Minipool_PromotionScrubPeriod, proposeSetting),
		utils.CreateBoolSetter("is-scrub-penalty-enabled", "ispe", minipoolContract, oracle.SettingName_Minipool_IsScrubPenaltyEnabled, proposeSetting),
		utils.CreateDurationSetter("bond-reduction-window-start", "brws", minipoolContract, oracle.SettingName_Minipool_BondReductionWindowStart, proposeSetting),
		utils.CreateDurationSetter("bond-reduction-window-length", "brwl", minipoolContract, oracle.SettingName_Minipool_BondReductionWindowLength, proposeSetting),
		utils.CreatePercentSetter("bond-reduction-cancellation-quorum", "brcq", minipoolContract, oracle.SettingName_Minipool_BondReductionCancellationQuorum, proposeSetting),
	}

	// Create the proposal settings commands
	proposalContract := rocketpool.ContractName_RocketDAONodeTrustedSettingsProposals
	proposalSettingsCmd := utils.CreateSetterCategory("proposal", "Proposal", "p", proposalContract)
	proposalSettingsCmd.Subcommands = []*cli.Command{
		utils.CreateDurationSetter("cooldown-time", "ct", proposalContract, oracle.SettingName_Proposal_CooldownTime, proposeSetting),
		utils.CreateDurationSetter("vote-time", "vt", proposalContract, oracle.SettingName_Proposal_VoteTime, proposeSetting),
		utils.CreateDurationSetter("vote-delay-time", "vdt", proposalContract, oracle.SettingName_Proposal_VoteDelayTime, proposeSetting),
		utils.CreateDurationSetter("execute-time", "et", proposalContract, oracle.SettingName_Proposal_ExecuteTime, proposeSetting),
		utils.CreateDurationSetter("action-time", "at", proposalContract, oracle.SettingName_Proposal_ActionTime, proposeSetting),
	}

	app.Commands = append(app.Commands, &cli.Command{
		Name:    name,
		Aliases: aliases,
		Usage:   "Manage the Rocket Pool oracle DAO",
		Subcommands: []*cli.Command{
			{
				Name:    "status",
				Aliases: []string{"s"},
				Usage:   "Get oracle DAO status",
				Action: func(c *cli.Context) error {
					// Validate args
					if err := input.ValidateArgCount(c, 0); err != nil {
						return err
					}

					// Run
					return getStatus(c)
				},
			},

			{
				Name:    "members",
				Aliases: []string{"m"},
				Usage:   "Get the oracle DAO members",
				Action: func(c *cli.Context) error {
					// Validate args
					if err := input.ValidateArgCount(c, 0); err != nil {
						return err
					}

					// Run
					return getMembers(c)
				},
			},

			{
				Name:    "settings",
				Aliases: []string{"e"},
				Usage:   "Get the oracle DAO settings",
				Action: func(c *cli.Context) error {
					// Run
					return getSettings(c)
				},
			},

			{
				Name:    "propose",
				Aliases: []string{"p"},
				Usage:   "Make an oracle DAO proposal",
				Subcommands: []*cli.Command{
					{
						Name:    "member",
						Aliases: []string{"m"},
						Usage:   "Make an oracle DAO member proposal",
						Subcommands: []*cli.Command{
							{
								Name:      "invite",
								Aliases:   []string{"i"},
								Usage:     "Propose inviting a new member",
								ArgsUsage: "member-address member-id member-url",
								Action: func(c *cli.Context) error {
									// Validate args
									if err := input.ValidateArgCount(c, 3); err != nil {
										return err
									}
									memberAddress, err := input.ValidateAddress("member address", c.Args().Get(0))
									if err != nil {
										return err
									}
									memberId, err := input.ValidateDAOMemberID("member ID", c.Args().Get(1))
									if err != nil {
										return err
									}

									// Run
									return proposeInvite(c, memberAddress, memberId, c.Args().Get(2))
								},
							},

							{
								Name:    "leave",
								Aliases: []string{"l"},
								Usage:   "Propose leaving the oracle DAO",
								Action: func(c *cli.Context) error {
									// Validate args
									if err := input.ValidateArgCount(c, 0); err != nil {
										return err
									}

									// Run
									return proposeLeave(c)
								},
							},

							{
								Name:    "kick",
								Aliases: []string{"k"},
								Usage:   "Propose kicking a member",
								Flags: []cli.Flag{
									utils.InstantiateFlag(memberFlag, "The address of the member to propose kicking"),
									kickFineFlag,
								},
								Action: func(c *cli.Context) error {
									// Validate args
									if err := input.ValidateArgCount(c, 0); err != nil {
										return err
									}

									// Validate flags
									if c.String(memberFlag.Name) != "" {
										if _, err := input.ValidateAddress(memberFlag.Name, c.String(memberFlag.Name)); err != nil {
											return err
										}
									}
									if c.String(kickFineFlag.Name) != "" && c.String(kickFineFlag.Name) != "max" {
										if _, err := input.ValidatePositiveEthAmount(kickFineFlag.Name, c.String(kickFineFlag.Name)); err != nil {
											return err
										}
									}

									// Run
									return proposeKick(c)
								},
							},
						},
					},

					{
						Name:    "setting",
						Aliases: []string{"s"},
						Usage:   "Make an oracle DAO setting proposal",
						Subcommands: []*cli.Command{
							memberSettingsCmd,
							minipoolSettingsCmd,
							proposalSettingsCmd,
						},
					},
				},
			},

			{
				Name:    "proposals",
				Aliases: []string{"o"},
				Usage:   "Manage oracle DAO proposals",
				Subcommands: []*cli.Command{

					{
						Name:    "list",
						Aliases: []string{"l"},
						Usage:   "List the oracle DAO proposals",
						Flags: []cli.Flag{
							proposalStatesFlag,
						},
						Action: func(c *cli.Context) error {
							// Validate args
							if err := input.ValidateArgCount(c, 0); err != nil {
								return err
							}

							// Run
							return getProposals(c, c.String(proposalStatesFlag.Name))
						},
					},

					{
						Name:      "details",
						Aliases:   []string{"d"},
						Usage:     "View proposal details",
						ArgsUsage: "proposal-id",
						Action: func(c *cli.Context) error {
							// Validate args
							var err error
							if err = input.ValidateArgCount(c, 1); err != nil {
								return err
							}
							id, err := input.ValidateUint("proposal-id", c.Args().Get(0))
							if err != nil {
								return err
							}

							// Run
							return getProposal(c, id)
						},
					},

					{
						Name:      "cancel",
						Aliases:   []string{"c"},
						Usage:     "Cancel a proposal made by the node",
						UsageText: "rocketpool odao proposals cancel [options]",
						Flags: []cli.Flag{
							utils.InstantiateFlag(proposalFlag, "The ID of the proposal to cancel"),
						},
						Action: func(c *cli.Context) error {
							// Validate args
							if err := input.ValidateArgCount(c, 0); err != nil {
								return err
							}

							// Run
							return cancelProposal(c)
						},
					},

					{
						Name:    "vote",
						Aliases: []string{"v"},
						Usage:   "Vote on a proposal",
						Flags: []cli.Flag{
							utils.InstantiateFlag(proposalFlag, "The ID of the proposal to vote on"),
							voteSupportFlag,
							utils.YesFlag,
						},
						Action: func(c *cli.Context) error {
							// Validate args
							if err := input.ValidateArgCount(c, 0); err != nil {
								return err
							}

							// Validate flags
							if c.String(voteSupportFlag.Name) != "" {
								if _, err := input.ValidateBool("support", c.String(voteSupportFlag.Name)); err != nil {
									return err
								}
							}

							// Run
							return voteOnProposal(c)
						},
					},

					{
						Name:    "execute",
						Aliases: []string{"x"},
						Usage:   "Execute a proposal",
						Flags: []cli.Flag{
							proposalFlag,
						},
						Action: func(c *cli.Context) error {
							// Validate args
							if err := input.ValidateArgCount(c, 0); err != nil {
								return err
							}

							// Validate flags
							if c.String(executeProposalFlag.Name) != "" && c.String(executeProposalFlag.Name) != "all" {
								if _, err := input.ValidatePositiveUint("proposal ID", c.String(executeProposalFlag.Name)); err != nil {
									return err
								}
							}

							// Run
							return executeProposal(c)
						},
					},
				},
			},

			{
				Name:      "join",
				Aliases:   []string{"j"},
				Usage:     "Join the oracle DAO (requires an executed invite proposal)",
				UsageText: "rocketpool odao join [options]",
				Flags: []cli.Flag{
					utils.YesFlag,
					joinSwapFlag,
				},
				Action: func(c *cli.Context) error {
					// Validate args
					if err := input.ValidateArgCount(c, 0); err != nil {
						return err
					}

					// Run
					return join(c)
				},
			},

			{
				Name:      "leave",
				Aliases:   []string{"l"},
				Usage:     "Leave the oracle DAO (requires an executed leave proposal)",
				UsageText: "rocketpool odao leave [options]",
				Flags: []cli.Flag{
					leaveRefundAddressFlag,
					utils.YesFlag,
				},
				Action: func(c *cli.Context) error {

					// Validate args
					if err := input.ValidateArgCount(c, 0); err != nil {
						return err
					}

					// Validate flags
					if c.String(leaveRefundAddressFlag.Name) != "" && c.String(leaveRefundAddressFlag.Name) != "node" {
						if _, err := input.ValidateAddress("bond refund address", c.String(leaveRefundAddressFlag.Name)); err != nil {
							return err
						}
					}

					// Run
					return leave(c)
				},
			},
		},
	})
}
