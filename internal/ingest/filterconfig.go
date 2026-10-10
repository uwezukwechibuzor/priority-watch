package ingest

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// poolsFile is the structure of config/pools.yaml.
type poolsFile struct {
	Pools []struct {
		Address string `yaml:"address"`
	} `yaml:"pools"`
}

// contractsFile is the structure of config/contracts.yaml.
type contractsFile struct {
	Contracts []struct {
		Address string `yaml:"address"`
	} `yaml:"contracts"`
}

// LoadLogFilterAddresses reads config/pools.yaml and config/contracts.yaml and
// returns a deduplicated, lowercased list of addresses to use in eth_getLogs
// filters.  An empty list means "all addresses" (no address filter applied).
func LoadLogFilterAddresses(poolsPath, contractsPath string) ([]string, error) {
	seen := make(map[string]struct{})
	var out []string

	if err := loadPoolAddresses(poolsPath, seen, &out); err != nil {
		return nil, fmt.Errorf("load pool addresses: %w", err)
	}
	if err := loadContractAddresses(contractsPath, seen, &out); err != nil {
		return nil, fmt.Errorf("load contract addresses: %w", err)
	}
	return out, nil
}

func loadPoolAddresses(path string, seen map[string]struct{}, out *[]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var f poolsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return err
	}
	for _, p := range f.Pools {
		addr := strings.ToLower(p.Address)
		if addr == "" {
			continue
		}
		if _, dup := seen[addr]; !dup {
			seen[addr] = struct{}{}
			*out = append(*out, addr)
		}
	}
	return nil
}

func loadContractAddresses(path string, seen map[string]struct{}, out *[]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var f contractsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return err
	}
	for _, c := range f.Contracts {
		addr := strings.ToLower(c.Address)
		if addr == "" {
			continue
		}
		if _, dup := seen[addr]; !dup {
			seen[addr] = struct{}{}
			*out = append(*out, addr)
		}
	}
	return nil
}
