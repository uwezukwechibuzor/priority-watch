import { check, sleep } from "k6";
import http from "k6/http";

export const options = {
  scenarios: {
    cached_endpoints: {
      executor: "constant-arrival-rate",
      rate: 50,
      timeUnit: "1s",
      duration: "60s",
      preAllocatedVUs: 20,
      maxVUs: 50,
    },
  },
  thresholds: {
    "http_req_duration{endpoint:blocks_kpi}": ["p(95)<300"],
    "http_req_duration{endpoint:health}": ["p(95)<300"],
    "http_req_duration{endpoint:treasury}": ["p(95)<300"],
    http_req_failed: ["rate<0.01"],
  },
};

const BASE = __ENV.API_BASE || "http://localhost:8080";

export default function () {
  const blocksKpi = http.get(`${BASE}/api/v1/blocks/kpi?range=7d`, {
    tags: { endpoint: "blocks_kpi" },
  });
  check(blocksKpi, {
    "blocks/kpi 200": (r) => r.status === 200,
  });

  const health = http.get(`${BASE}/api/v1/blocks/health?window=30d`, {
    tags: { endpoint: "health" },
  });
  check(health, {
    "health 200": (r) => r.status === 200,
  });

  const treasury = http.get(`${BASE}/api/v1/treasury?window=30d`, {
    tags: { endpoint: "treasury" },
  });
  check(treasury, {
    "treasury 200": (r) => r.status === 200,
  });

  sleep(0.05);
}
