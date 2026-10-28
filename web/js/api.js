/* 后端调用：宿主内走 $gm.request('/api/command/exec_shell')，浏览器内走开发桥接，都没有则回退 mock */
(function (global) {
  'use strict';

  var DEFAULT_IDENTITY = { user_id: 'jg', app_name: 'sshFootprint' };
  var identityPromise = null;
  var state = { usingMock: false, hasBridge: null };

  function loadIdentity() {
    if (identityPromise) return identityPromise;
    identityPromise = fetch('./liteapp.json', { cache: 'no-store' })
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (j) {
        return {
          user_id: (j && j.user_id) || DEFAULT_IDENTITY.user_id,
          app_name: (j && j.app_name) || DEFAULT_IDENTITY.app_name,
        };
      })
      .catch(function () { return { user_id: DEFAULT_IDENTITY.user_id, app_name: DEFAULT_IDENTITY.app_name }; });
    return identityPromise;
  }

  function quote(v) {
    return "'" + String(v).replace(/'/g, "'\\''") + "'";
  }

  function gm() {
    try { return global.$gm || null; } catch (e) { return null; }
  }

  function sendExec(payload) {
    var sdk = gm();
    if (sdk && sdk.request) return sdk.request('/api/command/exec_shell', payload);
    return fetch('/api/command/exec_shell', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(payload),
    }).then(function (r) {
      return r.json().then(function (j) {
        if (!r.ok) throw new Error((j && j.msg) || ('dev bridge HTTP ' + r.status));
        return j;
      });
    });
  }

  function unwrap(resp, binPath) {
    if (!resp) throw new Error('exec_shell: response is empty');

    if (typeof resp.code === 'number' && resp.code !== 200000 && resp.code !== 0) {
      throw new Error(resp.msg || resp.message || 'exec_shell failed');
    }

    var text = '';
    if (typeof resp.data === 'string') text = resp.data;
    else if (resp.data && typeof resp.data.stdout === 'string') text = resp.data.stdout;
    else if (typeof resp.stdout === 'string') text = resp.stdout;
    else if (typeof resp === 'string') text = resp;

    var stderr = '';
    if (resp.data && typeof resp.data.stderr === 'string') stderr = resp.data.stderr;
    else if (resp.stderr) stderr = resp.stderr;

    var trimmed = text.trim();
    if (!trimmed) {
      if (stderr.trim()) throw new Error('CLI 输出为空，stderr: ' + stderr.trim());
      throw new Error('CLI 输出为空，请确认 ' + binPath + ' 已部署且可执行');
    }

    var parsed;
    try {
      parsed = JSON.parse(trimmed);
    } catch (e) {
      var m = trimmed.match(/\{[\s\S]*\}/);
      if (!m) throw new Error('CLI 非 JSON 输出: ' + trimmed.slice(0, 160));
      try { parsed = JSON.parse(m[0]); }
      catch (e2) { throw new Error('CLI 非 JSON 输出: ' + trimmed.slice(0, 160)); }
    }

    if (parsed.ok === false) {
      throw new Error((parsed.error && (parsed.error.message || parsed.error.key)) || 'CLI 执行失败');
    }
    return parsed.data;
  }

  function callCli(action, args) {
    return loadIdentity().then(function (id) {
      var sdk = gm();
      var home = sdk && typeof sdk.gmsshHome === 'string' ? sdk.gmsshHome.replace(/\/$/, '') : '';
      var bin = '/.__gmssh/plugin/liteapp/' + id.user_id + '/' + id.app_name + '/app/bin/main';
      if (home) bin = bin.replace('/.__gmssh', home);

      var all = { action: action };
      if (args) {
        Object.keys(args).forEach(function (k) {
          if (args[k] !== undefined && args[k] !== '') all[k] = args[k];
        });
      }
      all.lang = (sdk && sdk.lang) || 'zh-CN';

      var flags = Object.keys(all).map(function (k) {
        return '--' + k + ' ' + quote(all[k]);
      }).join(' ');

      var cmd = quote(bin) + ' ssh_footprint ' + flags + ' --json';
      var payload = { method: 'post', data: { cmd_string: cmd, timeout: '35', cwd: '', user: '' } };

      var timeout = new Promise(function (_, reject) {
        setTimeout(function () { reject(new Error('后端执行超时（30s）')); }, 30000);
      });

      return Promise.race([sendExec(payload), timeout]).then(function (resp) {
        return unwrap(resp, bin);
      });
    });
  }

  // 是否没有宿主也没有开发桥接 —— 第三次探测失败后固定走 mock，避免每次扫描都等超时
  function shouldMock() {
    if (gm() && gm().request) return false;
    if (state.hasBridge === false) return true;
    return false;
  }

  function probe() {
    if (shouldMock()) {
      state.usingMock = true;
      return Promise.resolve(global.MOCK_PROBE);
    }
    return callCli('probe', null).then(function (d) {
      state.usingMock = false;
      state.hasBridge = true;
      return d;
    }).catch(function (err) {
      // 浏览器里没有 dev bridge（或后端没编译）时回退到模拟数据
      state.hasBridge = false;
      state.usingMock = true;
      return global.MOCK_PROBE;
    });
  }

  function scan(range) {
    if (shouldMock()) {
      state.usingMock = true;
      return Promise.resolve(global.getMockScan(range));
    }
    return callCli('scan', { range: range }).then(function (d) {
      state.usingMock = false;
      state.hasBridge = true;
      return d;
    }).catch(function (err) {
      state.hasBridge = false;
      state.usingMock = true;
      return global.getMockScan(range);
    });
  }

  global.API = {
    probe: probe,
    scan: scan,
    isMock: function () { return state.usingMock; },
  };
})(window);
