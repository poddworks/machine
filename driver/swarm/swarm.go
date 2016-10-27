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

			if len(managers) == 0 {
				return cli.NewExitError("You must specify at least one Manager Node", 1)
			}

			firstName := managers[0]
			firstManager, ok := mach.InstList[firstName]
			if !ok {
				return cli.NewExitError("Manager node not found", 1)
			}

			// Step 1: Connect to one manager and enable swarm mode
			advertiseAddr, err := firstManager.SwarmInit()
			if err != nil {
				return cli.NewExitError(fmt.Sprintf("%s - %s", firstName, err), 1)
			}

			// Step 2: Request a join token (manager and worker token)
			managerToken, workerToken, err := firstManager.SwarmToken()
			if err != nil {
				return cli.NewExitError("error/failed-to-create-swarm-token", 1)
			}

			// Step 3: Join manager nodes
			for _, name := range managers[1:] {
				node, ok := mach.InstList[name]
				if !ok {
					return cli.NewExitError("Manager node not found", 1)
				} else {
					node.NewDockerClient()
				}
				if err := node.SwarmJoin(managerToken, advertiseAddr); err != nil {
					return cli.NewExitError(fmt.Sprintf("%s - %s", name, err), 1)
				}
			}

			// Step 4: Join worker nodes
			for _, name := range workers {
				node, ok := mach.InstList[name]
				if !ok {
					return cli.NewExitError("error/manager-node-not-found", 1)
				}
				if err := node.SwarmJoin(workerToken, advertiseAddr); err != nil {
					return cli.NewExitError("error/failed-to-join-node", 1)
				}
			}

			return nil
		},
	}
}

func NewCommand() cli.Command {
	return cli.Command{
		Name:  "swarm",
		Usage: "Manage Docker Engine Swarm",
		Subcommands: []cli.Command{
			joinToCluster(),
		},
		BashComplete: func(c *cli.Context) {
			for _, cmd := range c.App.Commands {
				fmt.Fprint(c.App.Writer, " ", cmd.Name)
			}
		},
	}
}

func joinToCluster() cli.Command {
	return cli.Command{
		Name:  "join",
		Usage: "Join to a Swarm mode cluster",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "as-manager", Usage: "Join as manager"},
			cli.StringFlag{Name: "manager", Usage: "Manager of this cluster"},
		},
		Action: func(c *cli.Context) error {
			var (
				managerName = c.String("manager")

				newNodes = c.Args()

				asManager = c.Bool("as-manager")

				joinToken string
			)

			manager, ok := mach.InstList[managerName]
			if !ok {
				return cli.NewExitError("Manager node not found", 1)
			} else {
				manager.NewDockerClient()
			}

			// Step 1: Retrieve advertiseAddr from manager
			advertiseAddr := manager.AltHost[0]

			// Step 2: Request a join token (manager and worker token)
			managerToken, workerToken, err := manager.SwarmToken()
			if err != nil {
				return cli.NewExitError("error/failed-to-create-swarm-token", 1)
			}
			if asManager {
				joinToken = managerToken
			} else {
				joinToken = workerToken
			}

			for _, name := range newNodes {
				node, ok := mach.InstList[name]
				if !ok {
					return cli.NewExitError("Swarm node not found", 1)
				} else {
					node.NewDockerClient()
				}
				if err := node.SwarmJoin(joinToken, advertiseAddr); err != nil {
					return cli.NewExitError(fmt.Sprintf("%s - %s", name, err), 1)
				}
			}

			return nil
		},
		BashComplete: func(c *cli.Context) {
			for name, _ := range mach.InstList {
				fmt.Fprint(c.App.Writer, name, " ")
			}
		},
	}
}
