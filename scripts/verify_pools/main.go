//go:build ignore

// verify_pools is a standalone script that validates config/pools.yaml entries
// against on-chain state via JSON-RPC.
//
// Usage:
//
//	ARBITRUM_RPC_URL=https://arb1.arbitrum.io/rpc go run scripts/verify_pools/main.go
//
// For each pool with a non-empty address the script calls:
//   - token0()         selector 0x0dfe1681
//   - token1()         selector 0xd21220a7
//   - fee()            selector 0xddca3f43 (Uniswap V3 only)
//
// For each token it calls:
//   - symbol()         selector 0x95d89b41
//   - decimals()       selector 0x313ce567
//
// For Camelot V3 entries with an empty address the script calls the Algebra
// factory poolByPair(token0, token1) selector 0xd9a641e1 to discover the address.
//
// Results are printed as PASS/FAIL per pool.
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// poolTokenConfig mirrors the YAML token sub-struct.
type poolTokenConfig struct {
	Address  string `yaml:"address"`
	Symbol   string `yaml:"symbol"`
	Decimals uint8  `yaml:"decimals"`
}

// poolConfig mirrors config/pools.yaml pool entries.
type poolConfig struct {
	Address     string          `yaml:"address"`
	DEX         string          `yaml:"dex"`
	Token0      poolTokenConfig `yaml:"token0"`
	Token1      poolTokenConfig `yaml:"token1"`
	Fee         uint32          `yaml:"fee"`
	TickSpacing int32           `yaml:"tick_spacing"`
	Verified    bool            `yaml:"verified"`
	SourceURL   string          `yaml:"source_url"`
}

type poolsFile struct {
	Pools []poolConfig `yaml:"pools"`
}

// Algebra factory address (needs verification).
// Source: Camelot V3 docs — verify before use.
const algebraFactory = "0x1a3c9B1d2F0529D97f2afC5136Cc23e58f1FD35d"

func main() {
	rpcURL := os.Getenv("ARBITRUM_RPC_URL")
	if rpcURL == "" {
		fmt.Fprintln(os.Stderr, "ARBITRUM_RPC_URL is not set")
		os.Exit(1)
	}

	data, err := os.ReadFile("config/pools.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read config/pools.yaml: %v\n", err)
		os.Exit(1)
	}
	var f poolsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		fmt.Fprintf(os.Stderr, "parse config/pools.yaml: %v\n", err)
		os.Exit(1)
	}

	anyFail := false
	for i, pool := range f.Pools {
		fmt.Printf("\n--- Pool %d (dex=%s, source=%s) ---\n", i+1, pool.DEX, pool.SourceURL)

		if pool.Address == "" {
			if strings.Contains(pool.DEX, "camelot") {
				discovered, err := discoverCamelotPool(rpcURL, pool.Token0.Address, pool.Token1.Address)
				if err != nil {
					fmt.Printf("  SKIP discover address: %v\n", err)
					continue
				}
				fmt.Printf("  DISCOVERED address: %s\n", discovered)
				pool.Address = discovered
			} else {
				fmt.Printf("  SKIP (empty address)\n")
				continue
			}
		}

		pass := verifyPool(rpcURL, pool)
		if !pass {
			anyFail = true
		}
	}
	if anyFail {
		fmt.Println("\nSome checks FAILED — review output above.")
		os.Exit(1)
	}
	fmt.Println("\nAll checks PASSED.")
}

// verifyPool calls token0, token1, fee selectors and compares with YAML config.
// Returns true if all checks pass.
func verifyPool(rpcURL string, pool poolConfig) bool {
	pass := true

	// token0
	tok0, err := callAddress(rpcURL, pool.Address, "0x0dfe1681")
	if err != nil {
		fmt.Printf("  FAIL token0() call: %v\n", err)
		pass = false
	} else {
		cfg0 := strings.ToLower(pool.Token0.Address)
		if tok0 != cfg0 {
			fmt.Printf("  FAIL token0 mismatch: on-chain=%s yaml=%s\n", tok0, cfg0)
			pass = false
		} else {
			fmt.Printf("  PASS token0: %s\n", tok0)
		}
	}

	// token1
	tok1, err := callAddress(rpcURL, pool.Address, "0xd21220a7")
	if err != nil {
		fmt.Printf("  FAIL token1() call: %v\n", err)
		pass = false
	} else {
		cfg1 := strings.ToLower(pool.Token1.Address)
		if tok1 != cfg1 {
			fmt.Printf("  FAIL token1 mismatch: on-chain=%s yaml=%s\n", tok1, cfg1)
			pass = false
		} else {
			fmt.Printf("  PASS token1: %s\n", tok1)
		}
	}

	// fee (Uniswap V3 only)
	if strings.Contains(pool.DEX, "uniswap") {
		fee, err := callUint32(rpcURL, pool.Address, "0xddca3f43")
		if err != nil {
			fmt.Printf("  FAIL fee() call: %v\n", err)
			pass = false
		} else if fee != pool.Fee {
			fmt.Printf("  FAIL fee mismatch: on-chain=%d yaml=%d\n", fee, pool.Fee)
			pass = false
		} else {
			fmt.Printf("  PASS fee: %d\n", fee)
		}
	}

	// token0 symbol and decimals
	if tok0 != "" {
		sym, err := callString(rpcURL, tok0, "0x95d89b41")
		if err != nil {
			fmt.Printf("  WARN token0 symbol() call: %v\n", err)
		} else {
			fmt.Printf("  INFO token0 symbol: %s (yaml: %s)\n", sym, pool.Token0.Symbol)
		}
		dec, err := callUint8(rpcURL, tok0, "0x313ce567")
		if err != nil {
			fmt.Printf("  WARN token0 decimals() call: %v\n", err)
		} else if dec != pool.Token0.Decimals {
			fmt.Printf("  FAIL token0 decimals mismatch: on-chain=%d yaml=%d\n", dec, pool.Token0.Decimals)
			pass = false
		} else {
			fmt.Printf("  PASS token0 decimals: %d\n", dec)
		}
	}

	// token1 symbol and decimals
	if tok1 != "" {
		sym, err := callString(rpcURL, tok1, "0x95d89b41")
		if err != nil {
			fmt.Printf("  WARN token1 symbol() call: %v\n", err)
		} else {
			fmt.Printf("  INFO token1 symbol: %s (yaml: %s)\n", sym, pool.Token1.Symbol)
		}
		dec, err := callUint8(rpcURL, tok1, "0x313ce567")
		if err != nil {
			fmt.Printf("  WARN token1 decimals() call: %v\n", err)
		} else if dec != pool.Token1.Decimals {
			fmt.Printf("  FAIL token1 decimals mismatch: on-chain=%d yaml=%d\n", dec, pool.Token1.Decimals)
			pass = false
		} else {
			fmt.Printf("  PASS token1 decimals: %d\n", dec)
		}
	}

	if pass {
		fmt.Printf("  RESULT: PASS\n")
	} else {
		fmt.Printf("  RESULT: FAIL\n")
	}
	return pass
}

// discoverCamelotPool calls Algebra factory poolByPair(token0, token1)
// selector 0xd9a641e1 to find the pool address.
func discoverCamelotPool(rpcURL, token0, token1 string) (string, error) {
	// poolByPair(address,address) selector: 0xd9a641e1
	// Encode two 32-byte padded addresses.
	tok0 := strings.TrimPrefix(strings.ToLower(token0), "0x")
	tok1 := strings.TrimPrefix(strings.ToLower(token1), "0x")
	data := "0xd9a641e1" +
		fmt.Sprintf("%064s", tok0) +
		fmt.Sprintf("%064s", tok1)

	result, err := ethCall(rpcURL, algebraFactory, data)
	if err != nil {
		return "", fmt.Errorf("poolByPair call: %w", err)
	}
	// Result is a 32-byte padded address.
	addr, err := decodeAddress(result)
	if err != nil {
		return "", fmt.Errorf("decode poolByPair result: %w", err)
	}
	if addr == "0x0000000000000000000000000000000000000000" {
		return "", errors.New("pool not found (zero address returned)")
	}
	return addr, nil
}

// ── JSON-RPC helpers ──────────────────────────────────────────────────────────

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

type jsonRPCResponse struct {
	Result string `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ethCall sends eth_call with data to contract and returns the hex result.
func ethCall(rpcURL, contract, data string) (string, error) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_call",
		Params:  []any{map[string]string{"to": contract, "data": data}, "latest"},
		ID:      1,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(body)) //nolint:gosec,noctx
	if err != nil {
		return "", fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("rpc error: %s", rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

// callAddress calls a no-arg function that returns an address (32 bytes, last 20 used).
func callAddress(rpcURL, contract, selector string) (string, error) {
	result, err := ethCall(rpcURL, contract, selector)
	if err != nil {
		return "", err
	}
	return decodeAddress(result)
}

// decodeAddress parses a 0x-prefixed 32-byte hex into a checksummed address.
func decodeAddress(hex32 string) (string, error) {
	hex32 = strings.TrimPrefix(hex32, "0x")
	if len(hex32) < 64 {
		return "", fmt.Errorf("result too short: %q", hex32)
	}
	// Last 20 bytes of the 32-byte padded word.
	addr := "0x" + strings.ToLower(hex32[24:])
	return addr, nil
}

// callUint32 calls a no-arg function that returns a uint32 (32 bytes, last 4 used).
func callUint32(rpcURL, contract, selector string) (uint32, error) {
	result, err := ethCall(rpcURL, contract, selector)
	if err != nil {
		return 0, err
	}
	raw := strings.TrimPrefix(result, "0x")
	if len(raw) < 8 {
		return 0, fmt.Errorf("result too short: %q", result)
	}
	b, err := hex.DecodeString(raw[len(raw)-8:])
	if err != nil {
		return 0, fmt.Errorf("decode uint32: %w", err)
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

// callUint8 calls a no-arg function that returns a uint8.
func callUint8(rpcURL, contract, selector string) (uint8, error) {
	v, err := callUint32(rpcURL, contract, selector)
	if err != nil {
		return 0, err
	}
	return uint8(v), nil
}

// callString calls a no-arg function that returns a dynamic string (ABI-encoded).
func callString(rpcURL, contract, selector string) (string, error) {
	result, err := ethCall(rpcURL, contract, selector)
	if err != nil {
		return "", err
	}
	raw := strings.TrimPrefix(result, "0x")
	if len(raw) < 128 {
		// Fallback: try fixed 32-byte string encoding (older ERC-20 tokens).
		b, err := hex.DecodeString(raw)
		if err != nil {
			return "", fmt.Errorf("decode string: %w", err)
		}
		return strings.TrimRight(string(b), "\x00"), nil
	}
	// Standard ABI encoding: offset (32 bytes) + length (32 bytes) + data.
	lenHex := raw[64:128]
	strLenBytes, err := hex.DecodeString(lenHex)
	if err != nil {
		return "", fmt.Errorf("decode string length: %w", err)
	}
	strLen := 0
	for _, bb := range strLenBytes {
		strLen = strLen<<8 | int(bb)
	}
	if len(raw) < 128+strLen*2 {
		return "", fmt.Errorf("string data truncated")
	}
	strBytes, err := hex.DecodeString(raw[128 : 128+strLen*2])
	if err != nil {
		return "", fmt.Errorf("decode string data: %w", err)
	}
	return string(strBytes), nil
}
