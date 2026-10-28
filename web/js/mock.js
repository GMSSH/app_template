/* 仅在宿主的命令能力不可用时使用（界面会显示"模拟数据"角标） */
(function (global) {
  'use strict';

  global.MOCK_PROBE = {
    available: true,
    sources: ['systemd-journal', '/var/log/auth.log'],
    has_journal: true,
    readable_files: ['/var/log/auth.log'],
    permission: true,
    timezone: 'CST',
    timezone_desc: 'CST (UTC+08:00)',
    message: '日志源就绪（开发模拟数据）',
  };

  function pad(n) { return n < 10 ? '0' + n : '' + n; }

  global.getMockScan = function (range) {
    var now = Date.now();
    var bucketCount = 28;
    var stepMs = 6 * 3600 * 1000;
    if (range === '24h') { bucketCount = 24; stepMs = 3600 * 1000; }
    else if (range === '30d') { bucketCount = 30; stepMs = 24 * 3600 * 1000; }

    var buckets = [];
    var startMs = now - bucketCount * stepMs;

    for (var i = 0; i < bucketCount; i++) {
      var bStart = startMs + i * stepMs;
      var d = new Date(bStart);
      var label = '';
      if (range === '24h') label = pad(d.getHours()) + ':00';
      else if (range === '30d') label = pad(d.getMonth() + 1) + '-' + pad(d.getDate());
      else label = pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' + d.getHours() + 'h';

      var failed = 0;
      var success = 0;
      if (i % 4 === 1) failed = Math.floor(12 + Math.random() * 45);
      else if (i % 7 === 0) { failed = Math.floor(40 + Math.random() * 60); success = 1; }
      else if (i % 5 === 2) success = Math.floor(1 + Math.random() * 3);

      buckets.push({
        label: label,
        start_time: bStart,
        end_time: bStart + stepMs,
        failed_count: failed,
        success_count: success,
      });
    }

    var totalFailed = 0;
    var totalSuccess = 0;
    buckets.forEach(function (b) { totalFailed += b.failed_count; totalSuccess += b.success_count; });

    return {
      range: range,
      scan_time: new Date().toLocaleString('zh-CN'),
      timezone: 'CST',
      window_start: new Date(startMs).toLocaleString('zh-CN'),
      window_end: new Date(now).toLocaleString('zh-CN'),
      window_start_ts: startMs,
      window_end_ts: now,
      count_method: '按认证事件计数（开发模拟数据）',
      active_source: 'systemd-journal (ssh.service)',
      total_logs: 1842,
      summary: {
        total_failed: totalFailed,
        total_success: totalSuccess,
        unique_ips: 5,
      },
      buckets: buckets,
      ip_list: [
        {
          ip: '203.0.113.24',
          failed_count: 428,
          success_count: 0,
          users: ['root', 'admin', 'test'],
          first_seen: '2026-10-21 03:12:04',
          last_seen: '2026-10-23 21:40:12',
          first_seen_ts: now - 36 * 3600 * 1000,
          last_seen_ts: now - 30 * 60 * 1000,
          risk_level: 'high',
          risk_tags: ['高频登录尝试', '多账号尝试'],
          risk_reasons: ['5分钟内连续认证失败超过20次', '尝试了 3 个不同用户名 (admin, root, test)'],
          logs: [
            'Oct 23 21:40:12 srv sshd[1283]: Failed password for invalid user admin from 203.0.113.24 port 48102 ssh2',
            'Oct 23 21:40:10 srv sshd[1280]: Failed password for root from 203.0.113.24 port 48100 ssh2',
            'Oct 23 21:40:08 srv sshd[1278]: Failed password for test from 203.0.113.24 port 48098 ssh2',
            'Oct 23 21:40:06 srv sshd[1276]: Failed password for root from 203.0.113.24 port 48096 ssh2',
          ],
        },
        {
          ip: '198.51.100.88',
          failed_count: 16,
          success_count: 1,
          users: ['root'],
          first_seen: '2026-10-22 14:02:11',
          last_seen: '2026-10-22 14:15:20',
          first_seen_ts: now - 28 * 3600 * 1000,
          last_seen_ts: now - 27 * 3600 * 1000,
          risk_level: 'high',
          risk_tags: ['值得核对的成功登录'],
          risk_reasons: ['在该 IP 连续认证失败（含不存在的账号或多次更换账号）后，检测到了成功的 SSH 登录'],
          logs: [
            'Oct 22 14:15:20 srv sshd[8412]: Accepted password for root from 198.51.100.88 port 52310 ssh2',
            'Oct 22 14:14:55 srv sshd[8399]: Failed password for root from 198.51.100.88 port 52308 ssh2',
            'Oct 22 14:14:30 srv sshd[8380]: Failed password for root from 198.51.100.88 port 52304 ssh2',
          ],
        },
        {
          ip: '192.0.2.14',
          failed_count: 82,
          success_count: 0,
          users: ['deploy', 'guest', 'user', 'ubuntu'],
          first_seen: '2026-10-20 18:04:19',
          last_seen: '2026-10-23 11:20:00',
          first_seen_ts: now - 72 * 3600 * 1000,
          last_seen_ts: now - 8 * 3600 * 1000,
          risk_level: 'medium',
          risk_tags: ['多账号尝试', '反复认证失败'],
          risk_reasons: ['尝试了 4 个不同用户名 (deploy, guest, ubuntu, user)', '累计认证失败达 82 次'],
          logs: [
            'Oct 23 11:20:00 srv sshd[5211]: Failed password for invalid user guest from 192.0.2.14 port 38102 ssh2',
            'Oct 23 11:19:40 srv sshd[5209]: Failed password for ubuntu from 192.0.2.14 port 38100 ssh2',
          ],
        },
        {
          ip: '10.10.1.20',
          failed_count: 2,
          success_count: 0,
          users: ['root'],
          first_seen: '2026-10-23 09:10:00',
          last_seen: '2026-10-23 09:10:15',
          first_seen_ts: now - 12 * 3600 * 1000,
          last_seen_ts: now - 12 * 3600 * 1000,
          risk_level: 'low',
          risk_tags: ['偶发失败'],
          risk_reasons: ['存在 2 次偶发认证失败'],
          logs: [
            'Oct 23 09:10:15 srv sshd[2100]: Failed password for root from 10.10.1.20 port 61200 ssh2',
            'Oct 23 09:10:00 srv sshd[2098]: Failed password for root from 10.10.1.20 port 61198 ssh2',
          ],
        },
        {
          ip: '172.16.0.8',
          failed_count: 0,
          success_count: 14,
          users: ['deploy'],
          first_seen: '2026-10-18 10:00:00',
          last_seen: '2026-10-23 20:05:00',
          first_seen_ts: now - 120 * 3600 * 1000,
          last_seen_ts: now - 2 * 3600 * 1000,
          risk_level: 'info',
          risk_tags: ['正常登录'],
          risk_reasons: ['成功认证 14 次，无失败记录'],
          logs: [
            'Oct 23 20:05:00 srv sshd[1120]: Accepted publickey for deploy from 172.16.0.8 port 59120 ssh2: RSA SHA256:abc...',
          ],
        },
      ],
    };
  };
})(window);
