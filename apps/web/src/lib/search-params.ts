import { z } from "zod";

export const RANGES = ["1h", "24h", "7d", "30d", "all"] as const;
export type Range = (typeof RANGES)[number];

export const DEFAULT_RANGE: Range = "24h";

// /blocks search params
export const blocksSearchSchema = z.object({
  range: z.enum(RANGES).optional(),
  day: z.string().optional(),
  contested: z.boolean().optional(),
  payer: z.string().optional(),
  search: z.string().optional(),
  block: z.coerce.number().optional(),
  cursor: z.string().optional(),
});
export type BlocksSearch = z.infer<typeof blocksSearchSchema>;

// /health search params
export const healthSearchSchema = z.object({
  window: z.enum(["7d", "30d", "90d", "all"]).optional(),
  tab: z
    .enum(["overview", "trends", "payers", "treasury", "compare", "sources", "pipeline"])
    .optional(),
});
export type HealthSearch = z.infer<typeof healthSearchSchema>;

// /pricing search params
export const pricingSearchSchema = z.object({
  block: z.coerce.number().optional(),
  model: z.string().optional(),
  n: z.coerce.number().optional(),
  confidence: z.coerce.number().optional(),
});
export type PricingSearch = z.infer<typeof pricingSearchSchema>;

// /opportunity search params
export const opportunitySearchSchema = z.object({
  range: z.enum(RANGES).optional(),
  entity: z.string().optional(),
});
export type OpportunitySearch = z.infer<typeof opportunitySearchSchema>;

// /simulator search params
export const simulatorSearchSchema = z.object({
  strategy: z.enum(["fixed-tip", "percentile-of-recent", "spread-proportional"]).optional(),
  from_block: z.coerce.number().optional(),
  to_block: z.coerce.number().optional(),
});
export type SimulatorSearch = z.infer<typeof simulatorSearchSchema>;
