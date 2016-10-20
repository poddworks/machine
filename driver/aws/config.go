package aws

import (
	mach "github.com/poddworks/machine/lib/machine"

	"github.com/urfave/cli"

	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var (
	ErrFieldFromTag = fmt.Errorf("Unable to find field from tag")
)

func syncFromAWS() cli.Command {
	return cli.Command{
		Name:  "sync",
		Usage: "Sync AWS settings and assets",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "name", Value: "default", Usage: "Name of the profile"},
			cli.StringFlag{Name: "vpc-id", Value: "default", Usage: "AWS VPC identifier"},
			cli.BoolFlag{Name: "force,f", Usage: "Force new config file"},
		},
		Action: func(c *cli.Context) error {
			var forceNew = c.Bool("force")

			defer profile.Dump()
			defer mach.InstList.Dump()

			p := &Profile{Name: c.String("name"), Region: c.GlobalString("region")}
			if account_id, err := vpcInit(c, &p.VPC); err != nil {
				return cli.NewExitError(err.Error(), 1)
			} else {
				p.AccntId = account_id
			}
			if err := amiInit(c, p); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			if err := keypairInit(c, p); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			if err := ec2Init(forceNew); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			if _, ok := profile[p.Region]; !ok {
				profile[p.Region] = make(RegionProfile)
			}
			profile[p.Region][p.Name] = p
			return nil
		},
	}
}

func getFieldFromTag(t reflect.Type, s string) (f string, e error) {
	for idx := 0; idx < t.NumField(); idx++ {
		field := t.Field(idx)
		if tag := field.Tag.Get("json"); tag == s {
			return field.Name, nil
		}
	}
	return "", ErrFieldFromTag
}

func getFromAWSConfig() cli.Command {
	return cli.Command{
		Name:  "get",
		Usage: "Get config value from local store",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "name", Value: "default", Usage: "Name of the profile"},
		},
		Action: func(c *cli.Context) error {
			var (
				name   = c.String("name")
				region = c.GlobalString("region")

				qpath = c.Args().First()
			)

			// Retrieve user provide query path
			if qpath == "" {
				return nil // nothing to do here, abort
			}

			if _, ok := profile[region]; !ok {
				return cli.NewExitError("Selected region had no stored profile", 1)
			}

			if _, ok := profile[region][name]; !ok {
				return cli.NewExitError("Selected name had no profile", 1)
			}

			// TODO: report config value by dotted notation argument
			// e.g. .vpc.subnet.0.cidr
			var v interface{} = profile[region][name]

			for _, s := range strings.Split(qpath, ".") {
				val := reflect.ValueOf(v)

				// NOTE: dereference the value ahead of time
				if val.Kind() == reflect.Ptr {
					val = val.Elem()
				}

				switch val.Kind() {
				default:
					return cli.NewExitError(fmt.Sprintln("Path past leaf node -", qpath), 1)

				case reflect.Struct:
					if chk := val.FieldByName(s); chk.IsValid() {
						v = chk.Interface()
					} else if f, err := getFieldFromTag(val.Type(), s); err == nil {
						v = val.FieldByName(f).Interface()
					} else {
						return cli.NewExitError(fmt.Sprintln("invalid token -", s), 1)
					}

				case reflect.Slice:
					idx, err := strconv.Atoi(s)
					if err != nil {
						return cli.NewExitError(fmt.Sprintln("invalid token -", s), 1)
					}
					if idx < 0 || idx >= val.Len() {
						return cli.NewExitError(fmt.Sprintln("invalid token -", s), 1)
					}
					v = val.Index(idx).Interface()
				}
			}

			output, err := json.MarshalIndent(v, "", "    ")
			if err != nil {
				return cli.NewExitError(fmt.Sprintln("Corrupt profile -", name), 1)
			}

			var text = string(output)
			if unquoted, err := strconv.Unquote(text); err == nil {
				fmt.Println(unquoted)
			} else {
				fmt.Println(text)
			}

			return nil
		},
	}
}
