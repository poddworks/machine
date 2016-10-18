package swarm

import (
	mach "github.com/poddworks/machine/lib/machine"

	"github.com/urfave/cli"

	"fmt"
)

func NewCreateCommand() cli.Command {
	return cli.Command{
		Name:  "swarm",
		Usage: "Join Docker Engines into Swarm",
		Flags: []cli.Flag{
			cli.StringSliceFlag{Name: "manager", Usage: "Join Swarm as Manager"},
			cli.StringSliceFlag{Name: "worker", Usage: "Join Swarm as Worker"},
		},
		Action: func(c *cli.Context) error {
			var (
				managers = c.StringSlice("manager")
				workers  = c.StringSlice("worker")
			)

			_, certpath, err := mach.ParseCertArgs(c)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			if len(managers) == 0 {
				return cli.NewExitError("You must specify at least one Manager Node", 1)
			}

			firstName := managers[0]
			firstManager, ok := mach.InstList[firstName]
			if !ok {
				return cli.NewExitError("Manager node not found", 1)
			}

			// Step 1: Connect to one manager and enable swarm mode
			advertiseAddr, err := firstManager.SwarmInit(certpath)
			if err != nil {
				return cli.NewExitError(fmt.Sprintf("%s - %s", firstName, err), 1)
			}

			// Step 2: Request a join token (manager and worker token)
			managerToken, workerToken, err := firstManager.SwarmToken()
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			// Step 3: Join manager nodes
			for _, name := range managers[1:] {
				node, ok := mach.InstList[name]
				if !ok {
					return cli.NewExitError("Manager node not found", 1)
				} else {
					node.NewDockerClient(certpath)
				}
				if err := node.SwarmJoin(managerToken, advertiseAddr); err != nil {
					return cli.NewExitError(fmt.Sprintf("%s - %s", name, err), 1)
				}
			}

			// Step 4: Join worker nodes
			for _, name := range workers {
				node, ok := mach.InstList[name]
				if !ok {
					return cli.NewExitError("Manager node not found", 1)
				}
				if err := node.SwarmJoin(workerToken, advertiseAddr); err != nil {
					return cli.NewExitError(err.Error(), 1)
				}
			}

			return nil
		},
	}
}
