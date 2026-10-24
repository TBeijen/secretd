package cmd

import (
	"errors"
	"fmt"
	"strings"
)

func Ls(args []string) error {
	var name string
	switch {
	case len(args) == 0:
	case len(args) == 2 && args[0] == "--context":
		name = args[1]
	case len(args) == 1 && strings.HasPrefix(args[0], "--context="):
		name = strings.TrimPrefix(args[0], "--context=")
	default:
		return errors.New("usage: secretd ls [--context X]")
	}

	c, _, err := loadContext(name)
	if err != nil {
		return err
	}

	fmt.Printf("context: %s\n", c.Name)
	fmt.Printf("file:    %s\n", c.Path)
	if len(c.Match) > 0 {
		fmt.Printf("match:   %s\n", strings.Join(c.Match, ", "))
	}
	fmt.Println("profiles:")
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
			fmt.Printf("  %s  (%s)\n", n, strings.Join(extra, ", "))
		} else {
			fmt.Printf("  %s\n", n)
		}
	}
	return nil
}
