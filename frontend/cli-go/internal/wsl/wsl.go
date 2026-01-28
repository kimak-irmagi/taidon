package wsl

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Distro struct {
	Name    string
	Default bool
	State   string
	Version int
}

var (
	ErrNoDistros      = errors.New("no WSL distros found")
	ErrDistroNotFound = errors.New("requested WSL distro not found")
	ErrDistroAmbig    = errors.New("multiple WSL distros found")
)

// ParseDistroList parses `wsl --list --verbose` output.
func ParseDistroList(output string) ([]Distro, error) {
	lines := strings.Split(output, "\n")
	var distros []Distro
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "NAME") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		defaultMark := false
		nameIdx := 0
		if fields[0] == "*" {
			defaultMark = true
			nameIdx = 1
		}
		if len(fields) < nameIdx+3 {
			continue
		}
		version, err := strconv.Atoi(fields[nameIdx+2])
		if err != nil {
			continue
		}
		distros = append(distros, Distro{
			Name:    fields[nameIdx],
			Default: defaultMark,
			State:   fields[nameIdx+1],
			Version: version,
		})
	}
	if len(distros) == 0 {
		return nil, ErrNoDistros
	}
	return distros, nil
}

// SelectDistro chooses a distro based on preference and defaults.
func SelectDistro(distros []Distro, preferred string) (string, error) {
	if len(distros) == 0 {
		return "", ErrNoDistros
	}
	if preferred != "" {
		for _, d := range distros {
			if d.Name == preferred {
				return d.Name, nil
			}
		}
		return "", fmt.Errorf("%w: %s", ErrDistroNotFound, preferred)
	}
	if len(distros) == 1 {
		return distros[0].Name, nil
	}
	for _, d := range distros {
		if d.Default {
			return d.Name, nil
		}
	}
	return "", ErrDistroAmbig
}
