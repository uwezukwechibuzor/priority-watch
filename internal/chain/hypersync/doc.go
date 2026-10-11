// Package hypersync implements a chain.BlockSource backed by the Envio
// HyperSync query API for Arbitrum One (https://arbitrum.hypersync.xyz).
//
// HyperSync reliably provides the Arbitrum-specific gas_used_for_l1 field on
// receipts and is used as the fallback source that guarantees it.
package hypersync
