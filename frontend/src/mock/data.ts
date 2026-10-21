import type { PortEntry } from '../types/port'

const now = () => new Date().toISOString()
const p = (
  protocol: 'tcp' | 'udp', port: number, address: string, pid: number | null,
  processName: string, user: string, command: string, wd: string,
  ownerType: PortEntry['ownerType'] = 'process',
  service?: PortEntry['service'],
): PortEntry => ({
  protocol, port, address, state: protocol === 'tcp' ? 'LISTEN' : 'UNCONN', pid, processName, user,
  command, workingDirectory: wd, ownerType: pid === null ? 'unknown' : ownerType, ownerName: service?.serviceName || processName, scannedAt: now(),
  service,
})

let currentMockList: PortEntry[] = [
  // 1. Web 服务 (Nginx)
  p('tcp', 80, '0.0.0.0', 1320, 'nginx', 'root', 'nginx: master process /usr/sbin/nginx -c /etc/nginx/nginx.conf', '/etc/nginx', 'systemd', {
    category: 'web',
    serviceName: 'Nginx',
    summary: '高性能 HTTP 服务器与反向代理服务器',
    uptime: '14 天 6 小时',
    memoryRss: '48.5 MB',
    systemd: {
      unitName: 'nginx.service',
      description: 'A high performance web server and a reverse proxy server',
      activeState: 'active',
      subState: 'running',
      mainPid: 1320,
      execStart: '/usr/sbin/nginx -g daemon on; master_process on;',
      fragmentPath: '/lib/systemd/system/nginx.service',
      workingDirectory: '/etc/nginx',
      memoryCurrent: '48.5 MB',
      cpuUsageNSec: '12.45s',
    },
  }),
  p('tcp', 443, '0.0.0.0', 1320, 'nginx', 'root', 'nginx: master process /usr/sbin/nginx -c /etc/nginx/nginx.conf', '/etc/nginx', 'systemd', {
    category: 'web',
    serviceName: 'Nginx',
    summary: '高性能 HTTP 服务器与反向代理服务器',
    uptime: '14 天 6 小时',
    memoryRss: '48.5 MB',
    systemd: {
      unitName: 'nginx.service',
      description: 'A high performance web server and a reverse proxy server',
      activeState: 'active',
      subState: 'running',
      mainPid: 1320,
      execStart: '/usr/sbin/nginx -g daemon on; master_process on;',
      fragmentPath: '/lib/systemd/system/nginx.service',
      workingDirectory: '/etc/nginx',
      memoryCurrent: '48.5 MB',
      cpuUsageNSec: '12.45s',
    },
  }),

  // 2. 数据库 (MySQL)
  p('tcp', 3306, '127.0.0.1', 1188, 'mysqld', 'mysql', '/usr/sbin/mysqld --daemonize --pid-file=/run/mysqld/mysqld.pid', '/var/lib/mysql', 'systemd', {
    category: 'db',
    serviceName: 'MySQL',
    summary: '流行开源关系型数据库管理系统',
    uptime: '30 天 2 小时',
    memoryRss: '382.4 MB',
    systemd: {
      unitName: 'mysql.service',
      description: 'MySQL Community Server',
      activeState: 'active',
      subState: 'running',
      mainPid: 1188,
      execStart: '/usr/sbin/mysqld',
      fragmentPath: '/lib/systemd/system/mysql.service',
      workingDirectory: '/var/lib/mysql',
      memoryCurrent: '382.4 MB',
      cpuUsageNSec: '88.10s',
    },
  }),

  // 3. 数据库 (Redis)
  p('tcp', 6379, '127.0.0.1', 1204, 'redis-server', 'redis', 'redis-server 127.0.0.1:6379', '/var/lib/redis', 'systemd', {
    category: 'db',
    serviceName: 'Redis',
    summary: '高性能内存键值数据存储与缓存系统',
    uptime: '12 天 18 小时',
    memoryRss: '64.2 MB',
    systemd: {
      unitName: 'redis.service',
      description: 'Advanced key-value store',
      activeState: 'active',
      subState: 'running',
      mainPid: 1204,
      execStart: '/usr/bin/redis-server /etc/redis/redis.conf',
      fragmentPath: '/lib/systemd/system/redis.service',
      workingDirectory: '/var/lib/redis',
      memoryCurrent: '64.2 MB',
      cpuUsageNSec: '25.60s',
    },
  }),

  // 4. 容器服务 (gmssh-web / docker-proxy)
  p('tcp', 8080, '0.0.0.0', 3102, 'docker-proxy', 'root', '/usr/bin/docker-proxy -proto tcp -host-ip 0.0.0.0 -host-port 8080 -container-ip 172.17.0.2 -container-port 80', '/', 'docker', {
    category: 'container',
    serviceName: 'gmssh-web',
    summary: 'Docker 容器服务',
    uptime: '3 天 15 小时',
    memoryRss: '14.1 MB',
    docker: {
      containerId: 'e1f2a3c4b5d6',
      containerName: 'gmssh-web',
      image: 'nginx:latest',
      internalPort: '80',
      cmd: 'docker-entrypoint.sh nginx -g daemon off;',
    },
  }),

  // 5. 进程管理 (PM2 / Supervisor)
  p('tcp', 9001, '127.0.0.1', 2110, 'supervisord', 'root', '/usr/bin/python /usr/bin/supervisord -n -c /etc/supervisor/supervisord.conf', '/etc/supervisor', 'process', {
    category: 'manager',
    serviceName: 'Supervisor',
    summary: '基于 Python 的客户端/服务器进程管理系统',
    uptime: '8 天 4 小时',
    memoryRss: '28.3 MB',
  }),

  // 6. 未托管进程 (node / python)
  p('tcp', 3000, '0.0.0.0', 2481, 'node', 'www', 'node server.js --port 3000', '/www/project', 'process', {
    category: 'unmanaged',
    serviceName: 'node',
    summary: '基于 V8 引擎的 JavaScript 运行时环境',
    uptime: '1 天 9 小时',
    memoryRss: '124.6 MB',
  }),

  // 7. 系统服务 (sshd)
  p('tcp', 22, '0.0.0.0', 912, 'sshd', 'root', '/usr/sbin/sshd -D', '/', 'systemd', {
    category: 'system',
    serviceName: 'OpenSSH SSHD',
    summary: '安全 Shell 远程访问服务器守护进程',
    uptime: '45 天 12 小时',
    memoryRss: '8.4 MB',
    systemd: {
      unitName: 'ssh.service',
      description: 'OpenBSD Secure Shell server',
      activeState: 'active',
      subState: 'running',
      mainPid: 912,
      execStart: '/usr/sbin/sshd -D',
      fragmentPath: '/lib/systemd/system/ssh.service',
      workingDirectory: '/',
    },
  }),
]

export function mockScan(): PortEntry[] {
  return [...currentMockList]
}

export function mockReleasePort(port: number, protocol: string): boolean {
  const initialLength = currentMockList.length
  currentMockList = currentMockList.filter(item => !(item.port === port && item.protocol === protocol))
  return currentMockList.length < initialLength
}

export const delay = (ms: number) => new Promise((r) => setTimeout(r, ms))
