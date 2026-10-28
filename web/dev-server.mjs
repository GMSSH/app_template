// 本地联调服务器：静态文件 + /api/command/exec_shell 桥接（模拟宿主执行后端 CLI）
// 用法：node dev-server.mjs   →  http://dev.adde.cn:5185
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const ROOT = path.dirname(fileURLToPath(import.meta.url));
const PORT = 5185;
// 开发桥接直接调 go run，免去每次 make
const BACKEND_DIR = path.resolve(ROOT, '../backend');

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
};

function sendJson(res, code, obj) {
  res.writeHead(code, { 'content-type': 'application/json; charset=utf-8' });
  res.end(JSON.stringify(obj));
}

// 只接受 ssh_footprint 命令形态，参数做白名单校验后直接 spawn，不经过 shell
function parseCmd(cmd) {
  const m = cmd.match(/\sssh_footprint\s+(.+)$/);
  if (!m) return null;
  const args = m[1];
  if (!/^[A-Za-z0-9 _.,/'=:-]+--json$/.test(args)) return null;
  return args.split(/\s+/).map((s) => s.replace(/^'|'$/g, ''));
}

function execShell(req, res) {
  let raw = '';
  req.on('data', (c) => (raw += c));
  req.on('end', () => {
    let cmd = '';
    try {
      cmd = JSON.parse(raw)?.data?.cmd_string ?? '';
    } catch {
      return sendJson(res, 400, { code: 400, msg: 'bad json' });
    }
    const tokens = parseCmd(cmd);
    if (!tokens) return sendJson(res, 400, { code: 400, msg: 'dev bridge: unsupported cmd_string' });

    const child = spawn('go', ['run', './cmd/main', 'ssh_footprint', ...tokens], { cwd: BACKEND_DIR });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (d) => (stdout += d));
    child.stderr.on('data', (d) => (stderr += d));
    child.on('error', (e) => sendJson(res, 500, { code: 500, msg: 'spawn failed: ' + e.message }));
    child.on('close', () => sendJson(res, 200, { code: 200000, data: { stdout, stderr } }));
  });
}

const server = http.createServer((req, res) => {
  if (req.url.startsWith('/api/command/exec_shell')) {
    if (req.method !== 'POST') return sendJson(res, 405, { code: 405, msg: 'POST only' });
    return execShell(req, res);
  }

  const urlPath = decodeURIComponent(req.url.split('?')[0]);
  const rel = urlPath === '/' ? 'index.html' : urlPath.replace(/^\//, '');
  const file = path.join(ROOT, rel);

  if (!file.startsWith(ROOT)) {
    res.writeHead(403);
    return res.end('forbidden');
  }

  fs.readFile(file, (err, data) => {
    if (err) {
      res.writeHead(404, { 'content-type': 'text/plain; charset=utf-8' });
      return res.end('404 ' + rel);
    }
    res.writeHead(200, { 'content-type': MIME[path.extname(file)] || 'application/octet-stream' });
    res.end(data);
  });
});

server.listen(PORT, '0.0.0.0', () => {
  console.log('SSH 安全足迹（原生 HTML/CSS/JS）: http://dev.adde.cn:' + PORT);
  console.log('本地: http://localhost:' + PORT);
});
