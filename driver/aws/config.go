package aws

import (
	"github.com/codegangsta/cli"

	"encoding/json"
	"fmt"
	"os"
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
		},
		Action: func(c *cli.Context) error {
			var profile = make(AWSProfile)
			defer profile.Load().Dump()
			p := &Profile{Name: c.String("name"), Region: *sess.Config.Region}
			vpcInit(c, &p.VPC)
			amiInit(c, p)
			keypairInit(c, p)
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
				profile = make(AWSProfile)

				name   = c.String("name")
				region = *sess.Config.Region

				qpath = c.Args().First()
			)

			// Retrieve user provide query path
			if qpath == "" {
				return nil // nothing to do here, abort
			}

			profile.Load() // load stored local AWS config

			if _, ok := profile[region]; !ok {
				fmt.Fprintln(os.Stderr, "Selected region had no stored profile")
				os.Exit(1)
			}

			if _, ok := profile[region][name]; !ok {
				fmt.Fprintln(os.Stderr, "Selected name had no profile")
				os.Exit(1)
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
					fmt.Fprintln(os.Stderr, "Path past leaf node -", qpath)
					os.Exit(1)

				case reflect.Struct:
					if chk := val.FieldByName(s); chk.IsValid() {
						v = chk.Interface()
					} else if f, err := getFieldFromTag(val.Type(), s); err == nil {
						v = val.FieldByName(f).Interface()
					} else {
						fmt.Fprintln(os.Stderr, "invalid token -", s)
						os.Exit(1)
					}

				case reflect.Slice:
					idx, err := strconv.Atoi(s)
					if err != nil {
						fmt.Fprintln(os.Stderr, "invalid token -", s)
						os.Exit(1)
					}
					if idx < 0 || idx >= val.Len() {
						fmt.Fprintln(os.Stderr, "invalid token -", s)
						os.Exit(1)
					}
					v = val.Index(idx).Interface()
				}
			}

			if output, err := json.MarshalIndent(v, "", "    "); err != nil {
				fmt.Fprintln(os.Stderr, "Corrupt profile -", name)
				os.Exit(1)
			} else {
				fmt.Println(string(output))
			}

			return nil
		},
	}
}
