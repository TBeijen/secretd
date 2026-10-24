package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

const contextUsage = "usage: secretd context show [--context X] | context list"

func ContextCmd(args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New(contextUsage)
	}
	switch args[0] {
	case "show":
		return contextShow(args[1:], w)
	case "list":
		if len(args) > 1 {
			return errors.New(contextUsage)
		}
		return contextList(w)
	default:
		return fmt.Errorf("unknown context command %q, %s", args[0], contextUsage)
	}
}

func contextShow(args []string, w io.Writer) error {
	var name string
	switch {
	case len(args) == 0:
	case len(args) == 2 && args[0] == "--context":
		name = args[1]
	case len(args) == 1 && strings.HasPrefix(args[0], "--context="):
		name = strings.TrimPrefix(args[0], "--context=")
	default:
		return errors.New(contextUsage)
	}

	c, _, err := loadContext(name)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "context: %s\n", c.Name)
	fmt.Fprintf(w, "file:    %s\n", c.Path)
	if len(c.Match) > 0 {
		fmt.Fprintf(w, "match:   %s\n", strings.Join(c.Match, ", "))
	}
	fmt.Fprintln(w, "profiles:")
	for _, n := range c.ProfileNames() {
		p := c.AWS.Profiles[n]
		var extra []string
		if p.Source != "" {
			extra = append(extra, "source: "+p.Source)
		}
		if p.Region != "" {
			extra = append(extra, "region: "+p.Region)
		}
		if len(extra) > 0 {
			fmt.Fprintf(w, "  %s  (%s)\n", n, strings.Join(extra, ", "))
		} else {
			fmt.Fprintf(w, "  %s\n", n)
		}
	}
	return nil
}

func contextList(w io.Writer) error {
	contexts, _, err := loadContexts()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CONTEXT\tMATCH\tPROFILES")
	for _, c := range contexts {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", c.Name, orDash(strings.Join(c.Match, ", ")), strings.Join(c.ProfileNames(), ", "))
	}
	return tw.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
