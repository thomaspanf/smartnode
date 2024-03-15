package odao

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/rocket-pool/smartnode/rocketpool-cli/client"
	"github.com/rocket-pool/smartnode/rocketpool-cli/utils/tx"
)

func proposeLeave(c *cli.Context) error {
	// Get RP client
	rp, err := client.NewClientFromCtx(c).WithReady()
	if err != nil {
		return err
	}

	// Build the TX
	response, err := rp.Api.ODao.ProposeLeave()
	if err != nil {
		return err
	}

	// Verify
	if !response.Data.CanPropose {
		fmt.Println("Cannot propose leaving:")
		if response.Data.ProposalCooldownActive {
			fmt.Println("The node must wait for the proposal cooldown period to pass before making another proposal.")
		}
		if response.Data.InsufficientMembers {
			fmt.Println("There are not enough members in the oracle DAO to allow a member to leave.")
		}
		return nil
	}

	// Run the TX
	err = tx.HandleTx(c, rp, response.Data.TxInfo,
		"Are you sure you want to submit this proposal?",
		"proposing leaving Oracle DAO",
		"Proposing leaving the Oracle DAO...",
	)
	if err != nil {
		return err
	}

	// Log & return
	fmt.Println("Successfully submitted a leave proposal.")
	return nil

}
