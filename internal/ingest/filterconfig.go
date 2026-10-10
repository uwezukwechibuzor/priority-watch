package ingest

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// PoolTokenConfig holds token metadata within a pool entry.
type PoolTokenConfig struct {
	Address  string `yaml:"address"`
	Symbol   string `yaml:"symbol"`
	Decimals uint8  `yaml:"decimals"`
}

// PoolConfig holds rich metadata for a DEX liquidity pool.
type PoolConfig struct {
	Address     string          `yaml:"address"`
	DEX         string          `yaml:"dex"`
	EventType   string          `yaml:"event_type"`
	Token0      PoolTokenConfig `yaml:"token0"`
	Token1      PoolTokenConfig `yaml:"token1"`
	Fee         uint32          `yaml:"fee"`
	TickSpacing int32           `yaml:"tick_spacing"`
	CEXSymbol   string          `yaml:"cex_symbol"`
	Verified    bool            `yaml:"verified"`
	SourceURL   string          `yaml:"source_url"`
}

// ContractConfig holds a contract address with metadata.
type ContractConfig struct {
	Name          string `yaml:"name"`
	Address       string `yaml:"address"`
	ABIFile       string `yaml:"abi_file"`
	DeployedBlock uint64 `yaml:"deployed_block"`
	Verified      bool   `yaml:"verified"`
	SourceURL     string `yaml:"source_url"`
	Note          string `yaml:"note"`
}

// EntityAddress is a single address entry within an entity.
type EntityAddress struct {
	Address   string `yaml:"address"`
	Label     string `yaml:"label"`
	Verified  bool   `yaml:"verified"`
	SourceURL string `yaml:"source_url"`
}

// EntityConfig holds a named entity (searcher, MM, protocol) with addresses.
type EntityConfig struct {
	Name       string          `yaml:"name"`
	Type       string          `yaml:"type"`
	ColorIndex int             `yaml:"color_index"`
	Addresses  []EntityAddress `yaml:"addresses"`
	Note       string          `yaml:"note"`
	SourceURL  string          `yaml:"source_url"`
}

// poolsRichFile is the YAML structure for config/pools.yaml (rich metadata form).
type poolsRichFile struct {
	Pools []PoolConfig `yaml:"pools"`
}

// contractsRichFile is the YAML structure for config/contracts.yaml (rich metadata form).
type contractsRichFile struct {
	Contracts []ContractConfig `yaml:"contracts"`
}

// entitiesFile is the YAML structure for config/entities.yaml.
type entitiesFile struct {
	Entities []EntityConfig `yaml:"entities"`
}

// LoadPoolConfigs reads pools.yaml and returns the full pool metadata.
func LoadPoolConfigs(path string) ([]PoolConfig, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read pools config %s: %w", path, err)
	}
	var f poolsRichFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse pools config %s: %w", path, err)
	}
	return f.Pools, nil
}

// LoadContractConfigs reads contracts.yaml and returns the full contract metadata.
func LoadContractConfigs(path string) ([]ContractConfig, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read contracts config %s: %w", path, err)
	}
	var f contractsRichFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse contracts config %s: %w", path, err)
	}
	return f.Contracts, nil
}

// LoadEntityConfigs reads entities.yaml and returns the entity registry.
func LoadEntityConfigs(path string) ([]EntityConfig, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read entities config %s: %w", path, err)
	}
	var f entitiesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse entities config %s: %w", path, err)
	}
	return f.Entities, nil
}

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
	data, err := os.ReadFile(path) //nolint:gosec
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
	data, err := os.ReadFile(path) //nolint:gosec
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
