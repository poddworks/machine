package main

import (
	mach "github.com/poddworks/machine/lib/machine"

	"github.com/olekukonko/tablewriter"

	"fmt"
	"os"
	"regexp"
	"strings"
)

func match(name string, matchers []*regexp.Regexp) (matched bool) {
	if len(matchers) == 0 {
		matched = true
		return
	}
	for _, matcher := range matchers {
		if !matcher.MatchString(name) {
			// NOOP
		} else {
			matched = true
			return
		}
	}
	return
}

func listQuiet(matchers []*regexp.Regexp) {
	for name, _ := range mach.InstList {
		if !match(name, matchers) {
			continue
		}
		fmt.Print(name, " ")
	}
}

func listTable(matchers []*regexp.Regexp) {
	var (
		// Prepare table render
		table = tablewriter.NewWriter(os.Stdout)
	)

	table.SetBorder(false)

	table.SetHeader([]string{"", "Name", "DockerHost", "Driver", "State"})
	for name, inst := range mach.InstList {
		if !match(name, matchers) {
			continue
		}
		var dockerhost = inst.DockerHostName()
		var oneRow = []string{
			"",          // Current
			name,        // Name
			inst.Host,   // DockerHost
			inst.Driver, // Driver
			inst.State,  // State
		}
		if strings.Contains(os.Getenv("DOCKER_HOST"), dockerhost) {
			oneRow[0] = "*"
		}
		table.Append(oneRow)
	}

	table.Render()
}
