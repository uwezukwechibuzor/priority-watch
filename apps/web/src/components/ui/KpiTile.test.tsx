import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { KpiTile } from "./KpiTile";

describe("KpiTile", () => {
  it("renders label and value", () => {
    render(<KpiTile label="TIP REVENUE" value="1.234" unit="ETH" />);
    expect(screen.getByText("TIP REVENUE")).toBeInTheDocument();
    expect(screen.getByText("1.234")).toBeInTheDocument();
    expect(screen.getByText("ETH")).toBeInTheDocument();
  });

  it("renders subtext", () => {
    render(<KpiTile label="HHI" value="2,100" subtext="Moderate" />);
    expect(screen.getByText("Moderate")).toBeInTheDocument();
  });

  it("renders skeleton when loading", () => {
    const { container } = render(<KpiTile label="BLOCKS" loading />);
    // Should not show actual value, should show skeleton placeholders
    expect(container.querySelectorAll(".skeleton").length).toBeGreaterThan(0);
  });

  it("renders dash when value is undefined", () => {
    render(<KpiTile label="CONTESTED" />);
    expect(screen.getByText("—")).toBeInTheDocument();
  });
});
