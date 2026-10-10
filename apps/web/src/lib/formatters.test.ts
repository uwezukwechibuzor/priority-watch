import { describe, expect, it } from "vitest";
import {
  compact,
  entityColor,
  entityColorByName,
  formatGwei,
  formatPct,
  formatUsd,
  hhiBand,
  ratioBand,
  relativeTime,
  shortAddress,
  weiToEth,
} from "./formatters";

describe("weiToEth", () => {
  it("converts 1 ETH (1e18 wei)", () => {
    expect(weiToEth("1000000000000000000")).toBe("1.000000 ETH");
  });

  it("converts 0 wei", () => {
    expect(weiToEth("0")).toBe("0 ETH");
  });

  it("converts small amount to gwei fallback", () => {
    const result = weiToEth("100"); // 100 wei → 0.0000001 gwei-ish
    expect(result).toContain("gwei");
  });
});

describe("formatGwei", () => {
  it("formats large gwei value", () => {
    expect(formatGwei(1.5)).toBe("1.50 gwei");
  });

  it("formats sub-gwei value", () => {
    expect(formatGwei(0.001)).toBe("0.0010 gwei");
  });
});

describe("formatUsd", () => {
  it("formats millions", () => {
    expect(formatUsd(2_500_000)).toBe("$2.50M");
  });
  it("formats thousands", () => {
    expect(formatUsd(1500)).toBe("$1.50K");
  });
  it("formats dollars", () => {
    expect(formatUsd(3.5)).toBe("$3.50");
  });
  it("formats small amounts", () => {
    expect(formatUsd(0.001)).toBe("<$0.01");
  });
});

describe("compact", () => {
  it("formats millions", () => {
    expect(compact(1_234_567)).toBe("1.2M");
  });
  it("formats thousands", () => {
    expect(compact(1_234)).toBe("1.2K");
  });
  it("formats small numbers", () => {
    expect(compact(999)).toBe("999");
  });
});

describe("formatPct", () => {
  it("formats fraction to percent", () => {
    expect(formatPct(0.423)).toBe("42.3%");
  });
  it("handles 100%", () => {
    expect(formatPct(1.0)).toBe("100.0%");
  });
});

describe("shortAddress", () => {
  it("truncates a full address", () => {
    expect(shortAddress("0x1234567890abcdef1234567890abcdef12345678")).toBe("0x1234…5678");
  });
  it("returns short strings unchanged", () => {
    expect(shortAddress("short")).toBe("short");
  });
});

describe("relativeTime", () => {
  it("returns seconds for recent timestamps", () => {
    const ts = Date.now() - 5000;
    expect(relativeTime(ts)).toBe("5s ago");
  });
  it("returns minutes for older timestamps", () => {
    const ts = Date.now() - 120_000;
    expect(relativeTime(ts)).toBe("2m ago");
  });
  it("parses ISO strings", () => {
    const iso = new Date(Date.now() - 3000).toISOString();
    expect(relativeTime(iso)).toBe("3s ago");
  });
});

describe("hhiBand", () => {
  it("classifies competitive HHI", () => {
    expect(hhiBand(1000)).toBe("competitive");
  });
  it("classifies moderate HHI", () => {
    expect(hhiBand(2000)).toBe("moderate");
  });
  it("classifies high HHI", () => {
    expect(hhiBand(3000)).toBe("high");
  });
  it("boundary: 1500 is moderate", () => {
    expect(hhiBand(1500)).toBe("moderate");
  });
  it("boundary: 2500 is high", () => {
    expect(hhiBand(2500)).toBe("high");
  });
});

describe("ratioBand", () => {
  it("low", () => {
    expect(ratioBand(0.3)).toBe("low");
  });
  it("mid", () => {
    expect(ratioBand(0.6)).toBe("mid");
  });
  it("high", () => {
    expect(ratioBand(0.9)).toBe("high");
  });
});

describe("entityColor", () => {
  it("returns first colour for index 0", () => {
    expect(entityColor(0)).toBe("#3b82f6");
  });
  it("wraps around the palette", () => {
    expect(entityColor(8)).toBe("#3b82f6"); // wraps back to 0
  });
});

describe("entityColorByName", () => {
  it("returns a hex colour", () => {
    const c = entityColorByName("some-entity");
    expect(c).toMatch(/^#[0-9a-f]{6}$/);
  });
  it("is stable for the same name", () => {
    expect(entityColorByName("entity-A")).toBe(entityColorByName("entity-A"));
  });
});
