import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Custom metrics
const stockDepleted = new Counter('stock_depleted');
const realErrors = new Rate('real_errors');
const successLatency = new Trend('success_latency', true); // true = track percentiles

export const options = {
  stages: [
    { duration: '30s', target: 200 },
    { duration: '1m', target: 500 },
    { duration: '2m', target: 1000 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    // 99.9% of responses must be 200 or 429 (allow rare network glitches)
    checks: ['rate>=0.999'],
    // Only successful reservations must be fast (p99 < 150ms)
    success_latency: ['p(99)<150'],
    // Real errors (5xx) must be < 1%
    real_errors: ['rate<0.01'],
  },
};

export default function () {
  const payload = JSON.stringify({
    product_id: 1,
    user_id: `user-${__VU}-${__ITER}`,
  });
  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  const start = Date.now();
  const res = http.post('http://localhost:8080/reserve', payload, params);
  const end = Date.now();

  // Record latency only for successful (200) requests
  if (res.status === 200) {
    successLatency.add(end - start);
  }

  // Check acceptable status codes (200 or 429)
  check(res, {
    'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
  });

  // Track rate‑limited / sold‑out responses (both return 429)
  if (res.status === 429) {
    stockDepleted.add(1);
  }

  // Count only 5xx as real errors
  if (res.status >= 500) {
    realErrors.add(1);
  } else {
    realErrors.add(0);
  }

  sleep(0.1);
}