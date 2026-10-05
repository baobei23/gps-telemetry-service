import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

// Custom trends for granular per-endpoint latency monitoring
const listDevicesTrend = new Trend('lat_list_devices', true);
const getLatestTrend = new Trend('lat_get_latest', true);
const listPositionsP1Trend = new Trend('lat_list_positions_p1', true);
const listPositionsP2Trend = new Trend('lat_list_positions_p2', true);

export const options = {
  scenarios: {
    read_load_test: {
      executor: 'constant-vus',
      vus: 50, // 50 concurrent readers
      duration: '30s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.001'], // 99.9% success rate
    lat_get_latest: ['p(95)<50'], // 95% latest fixes under 50ms
    lat_list_positions_p1: ['p(95)<60'], // 95% page 1 reads under 60ms
    lat_list_positions_p2: ['p(95)<60'], // 95% cursor page 2 reads under 60ms
    lat_list_devices: ['p(95)<250'], // 95% 10k devices array serialization under 250ms
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:5055';

export default function () {
  // Random device from the 10,000 seeded dataset (357565514000001 - 357565514010000)
  const randomNum = Math.floor(Math.random() * 10000) + 1;
  const deviceId = `357565514${String(randomNum).padStart(6, '0')}`;

  // 1. GET /api/devices (sampled ~10% to simulate periodic dashboard fleet refresh)
  if (Math.random() < 0.1) {
    const resDevices = http.get(`${BASE_URL}/api/devices`, {
      tags: { name: '/api/devices' },
    });
    listDevicesTrend.add(resDevices.timings.duration);
    check(resDevices, {
      'list devices status is 200': (r) => r.status === 200,
      'list devices has data': (r) => r.json('data') && r.json('data').length > 0,
    });
  }

  // 2. GET /api/devices/{id}/latest
  const resLatest = http.get(`${BASE_URL}/api/devices/${deviceId}/latest`, {
    tags: { name: '/api/devices/:id/latest' },
  });
  getLatestTrend.add(resLatest.timings.duration);
  check(resLatest, {
    'get latest status is 200': (r) => r.status === 200,
    'latest deviceId matches': (r) => r.json('data.deviceId') === deviceId,
  });

  // 3. GET /api/devices/{id}/positions?limit=50 (Page 1)
  const resP1 = http.get(`${BASE_URL}/api/devices/${deviceId}/positions?limit=50`, {
    tags: { name: '/api/devices/:id/positions' },
  });
  listPositionsP1Trend.add(resP1.timings.duration);
  const okP1 = check(resP1, {
    'history p1 status is 200': (r) => r.status === 200,
    'history p1 has 50 items': (r) => r.json('data') && r.json('data').length === 50,
    'history p1 has nextCursor': (r) => r.json('pagination.nextCursor') !== undefined,
  });

  // 4. GET /api/devices/{id}/positions?limit=50&cursor={nextCursor} (Page 2 keyset traversal)
  if (okP1) {
    const nextCursor = resP1.json('pagination.nextCursor');
    if (nextCursor) {
      const resP2 = http.get(
        `${BASE_URL}/api/devices/${deviceId}/positions?limit=50&cursor=${encodeURIComponent(nextCursor)}`,
        { tags: { name: '/api/devices/:id/positions?cursor' } }
      );
      listPositionsP2Trend.add(resP2.timings.duration);
      check(resP2, {
        'history p2 status is 200': (r) => r.status === 200,
        'history p2 has 50 items': (r) => r.json('data') && r.json('data').length === 50,
      });
    }
  }

  // Pacing between user navigation
  sleep(0.05);
}
