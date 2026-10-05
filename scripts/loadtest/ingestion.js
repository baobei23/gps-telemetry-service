import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    telemetry_ingestion: {
      executor: 'constant-vus',
      vus: 100, // 100 simulated active devices
      duration: '30s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'], // 99% success rate
    http_req_duration: ['p(95)<150', 'p(99)<500'], // 95% requests < 150ms, 99% < 500ms
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:5055';

export default function () {
  // Each VU simulates a unique 15-digit IMEI GPS tracker
  const deviceId = `35756551452${String(__VU).padStart(4, '0')}`;
  const now = Math.floor(Date.now() / 1000);

  // Coordinate jitter around central Jakarta (-6.200000, 106.816666)
  const lat = (-6.200000 + (Math.random() - 0.5) * 0.01).toFixed(6);
  const lon = (106.816666 + (Math.random() - 0.5) * 0.01).toFixed(6);
  const speed = (15.0 + Math.random() * 5.0).toFixed(2);
  const bearing = Math.floor(Math.random() * 360);
  const batt = 85;

  const url = `${BASE_URL}/?id=${deviceId}&timestamp=${now}&lat=${lat}&lon=${lon}&valid=1&speed=${speed}&bearing=${bearing}&altitude=12.0&accuracy=5.0&hdop=0.80&batt=${batt}&charge=false`;

  const res = http.get(url);

  check(res, {
    'status is 200': (r) => r.status === 200,
  });

  // 1-second transmission interval per device (~100 req/s aggregate)
  sleep(1);
}
