import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';

const baseURL = ('https://cards-api-dev-alb-1463139058.us-east-2.elb.amazonaws.com').replace(/\/$/, '');

const instanceResponses = new Counter('instance_responses');

export const options = {
  vus: 5,
  duration: '30s',
  noConnectionReuse: true,
  thresholds: {
    http_req_failed: ['rate<0.01'],
    checks: ['rate>0.99'],
  },
};

export default function () {
  const response = http.get(`${baseURL}/health`, { tags: { name: 'health' } });
  let payload = {};

  try {
    payload = response.json();
  } catch (_) {
  }

  const instanceID = payload.instance_id || 'missing';
  instanceResponses.add(1, { instance_id: instanceID });

  check(response, {
    'health returns HTTP 200': (res) => res.status === 200,
    'health status is ok': () => payload.status === 'ok',
    'instance_id is present': () => Boolean(payload.instance_id),
  });

  sleep(0.5);
}
