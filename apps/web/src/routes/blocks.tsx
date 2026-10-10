import { BlockDrawer } from "@/components/blocks/BlockDrawer";
import { BlockTable } from "@/components/blocks/BlockTable";
import { BlocksKpiStrip } from "@/components/blocks/BlocksKpiStrip";
import { BlocksToolbar } from "@/components/blocks/BlocksToolbar";
import { PayerSharePanel } from "@/components/blocks/PayerSharePanel";
import { TipRevenueChart } from "@/components/blocks/TipRevenueChart";
import { TippedHistPanel } from "@/components/blocks/TippedHistPanel";
import { type BlocksSearch, blocksSearchSchema } from "@/lib/search-params";
import {
  type ListBlocksParams,
  blocksKpisQuery,
  blocksSeriesQuery,
  eventsQuery,
  payerShareQuery,
  tippedHistQuery,
} from "@/queries/blocks";
import { rootRoute } from "@/router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";

export const blocksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/blocks",
  validateSearch: (s) => blocksSearchSchema.parse(s),
  component: BlocksPage,
});

function BlocksPage() {
  const search = blocksRoute.useSearch();
  const navigate = useNavigate({ from: "/blocks" });
  const queryClient = useQueryClient();
  const searchInputRef = useRef<HTMLInputElement | null>(null);

  const range = search.range ?? "24h";
  const day = search.day;

  const { data: kpis, isLoading: kpisLoading } = useQuery(blocksKpisQuery(range, day));
  const { data: seriesData } = useQuery(blocksSeriesQuery(range));
  const series = seriesData?.data ?? [];

  const { data: payerData } = useQuery(payerShareQuery(range));
  const { data: histData } = useQuery(tippedHistQuery(range));
  const { data: eventsData } = useQuery(eventsQuery());

  const tableParams: ListBlocksParams = {
    range,
    day,
    search: search.search,
    entity: search.payer,
    contested_only: search.contested,
  };

  const [selectedBlock, setSelectedBlock] = useState<number | undefined>(search.block);

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (
        e.key === "/" &&
        !(e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement)
      ) {
        e.preventDefault();
        searchInputRef.current?.focus();
      }
      if (e.key === "Escape" && selectedBlock !== undefined) {
        setSelectedBlock(undefined);
        void navigate({ search: (prev) => ({ ...prev, block: undefined }) });
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [selectedBlock, navigate]);

  function openBlock(b: { block_number: number }) {
    setSelectedBlock(b.block_number);
    void navigate({ search: (prev) => ({ ...prev, block: b.block_number }) });
  }

  function closeBlock() {
    setSelectedBlock(undefined);
    void navigate({ search: (prev) => ({ ...prev, block: undefined }) });
  }

  function setRange(r: string) {
    void navigate({
      search: (prev) => ({ ...prev, range: r as BlocksSearch["range"], cursor: undefined }),
    });
  }

  function toggleContested(v: boolean) {
    void navigate({ search: (prev) => ({ ...prev, contested: v || undefined }) });
  }

  function setPayerFilter(v: string) {
    void navigate({ search: (prev) => ({ ...prev, payer: v || undefined }) });
  }

  function setSearchFilter(v: string) {
    void navigate({ search: (prev) => ({ ...prev, search: v || undefined }) });
  }

  function handleRefresh() {
    void queryClient.invalidateQueries({ queryKey: ["blocks"] });
  }

  return (
    <div style={{ padding: 16 }}>
      {/* KPI STRIP */}
      <BlocksKpiStrip kpis={kpis} series={series} loading={kpisLoading} />

      {/* TOOLBAR */}
      <BlocksToolbar
        range={range}
        onRangeChange={setRange}
        search={search.search ?? ""}
        onSearchChange={setSearchFilter}
        contested={search.contested ?? false}
        onContestedChange={toggleContested}
        payer={search.payer ?? ""}
        onPayerChange={setPayerFilter}
        onRefresh={handleRefresh}
        rows={[]}
        searchInputRef={searchInputRef}
      />

      {/* PANELS ROW */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "4fr 2fr 2fr",
          gap: 8,
          marginBottom: 8,
        }}
      >
        <TipRevenueChart series={series} events={eventsData?.data ?? null} />
        <PayerSharePanel
          data={payerData?.data ?? null}
          onPayerClick={(name) => setPayerFilter(name)}
        />
        <TippedHistPanel data={histData?.data ?? null} />
      </div>

      {/* BLOCK TABLE */}
      <BlockTable
        params={tableParams}
        range={range}
        selectedBlock={selectedBlock}
        onBlockSelect={openBlock}
        liveMode={range === "1h"}
      />

      {/* DRAWER */}
      <BlockDrawer blockNumber={selectedBlock} onClose={closeBlock} />
    </div>
  );
}
