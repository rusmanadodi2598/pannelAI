// Minimal API contract stub for browser tests. It exercises the real SvelteKit server and
// browser session flow without credentials, a provider account, PostgreSQL, or Redis.
const http = require('node:http');

const PORT = Number(process.env.MOCK_API_PORT || 4179);
const SESSION_COOKIE = 'panel_session=browser-e2e-ok';

function send(res, status, body, headers = {}) {
  res.writeHead(status, {
    'cache-control': 'no-store',
    ...headers,
    ...(body === undefined ? {} : { 'content-type': 'application/json; charset=utf-8' })
  });
  res.end(body === undefined ? undefined : JSON.stringify(body));
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url || '/', `http://127.0.0.1:${PORT}`);
  const authenticated = (req.headers.cookie || '').split(';').some((part) => part.trim() === SESSION_COOKIE);

  if (req.method === 'GET' && url.pathname === '/api/v1/auth/status') {
    return send(res, 200, {
      authenticated,
      require_login: !authenticated,
      password_configured: true
    });
  }

  if (req.method === 'POST' && url.pathname === '/api/v1/auth/login') {
    let body = '';
    req.setEncoding('utf8');
    req.on('data', (chunk) => { body += chunk; });
    req.on('end', () => {
      let password = '';
      try { password = JSON.parse(body).password || ''; } catch {}
      if (password !== 'browser-e2e-password') {
        return send(res, 401, {
          error: { code: 'UNAUTHORIZED', message: 'Invalid password.' }
        });
      }
      return send(res, 204, undefined, {
        'set-cookie': `${SESSION_COOKIE}; Path=/; HttpOnly; SameSite=Lax`
      });
    });
    return;
  }

  if (req.method === 'POST' && url.pathname === '/api/v1/auth/logout') {
    return send(res, 204, undefined, {
      'set-cookie': 'panel_session=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0'
    });
  }

  return send(res, 404, {
    error: { code: 'NOT_FOUND', message: 'This endpoint is not part of the browser smoke fixture.' }
  });
});

server.listen(PORT, '127.0.0.1', () => {
  process.stdout.write(`browser E2E API stub listening on 127.0.0.1:${PORT}\n`);
});
