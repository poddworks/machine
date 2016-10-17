package main

import (
	mach "github.com/poddworks/machine/lib/machine"

	"github.com/urfave/cli"

	"encoding/json"
	"fmt"
	"os"
)

func DnstoolCommand() cli.Command {
	return cli.Command{
		Name:  "dns",
		Usage: "Utility for quering DNS record",
		Subcommands: []cli.Command{
			{
				Name:  "lookup-srv",
				Usage: "Lookup SRV record",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "proto", Value: "tcp", Usage: "Service Protocol [tcp|udp]"},
					cli.BoolFlag{Name: "verbose", Usage: "Print more info"},
				},
				Action: func(c *cli.Context) error {
					var (
						verbose = c.Bool("verbose")

						proto = c.String("proto")

						srv, zone = c.Args().Get(0), c.Args().Get(1)
					)

					records, err := mach.LookupSRV(srv, proto, zone)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					} else {
						for _, r := range records {
							r.Target = r.Target[0 : len(r.Target)-1]
						}
					}

					if verbose {
						text, _ := json.MarshalIndent(records, "", "  ")
						fmt.Fprintf(os.Stdout, "%s\n", text)
					} else {
						for _, r := range records {
							fmt.Fprintf(os.Stdout, "%s ", r.Target)
						}
					}

					return nil
				},
			},
		},
		BashComplete: func(c *cli.Context) {
			for _, cmd := range c.App.Commands {
				fmt.Fprint(c.App.Writer, " ", cmd.Name)
			}
		},
	}
}
