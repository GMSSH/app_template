/* SSH 安全足迹 —— 主逻辑（原生 JS，无框架） */
(function (global) {
  'use strict';

  var state = {
    range: '7d',
    loading: true,
    scanData: null,
    selectedIp: null,
    query: '',
    error: '',
    tab: 'overview',
    tagFilter: '',
  };

  var el = {};

  function $(id) { return document.getElementById(id); }

  function init() {
    el.mockBadge = $('mockBadge');
    el.rangeControl = $('rangeControl');
    el.scanBtn = $('scanBtn');
    el.scanIcon = $('scanIcon');
    el.scanBtnText = $('scanBtnText');
    el.errorBanner = $('errorBanner');
    el.errorText = $('errorText');
    el.retryBtn = $('retryBtn');
    el.statFailed = $('statFailed');
    el.statSuccess = $('statSuccess');
    el.statIps = $('statIps');
    el.sectionCount = $('sectionCount');
    el.searchBox = $('searchBox');
    el.searchInput = $('searchInput');
    el.searchClear = $('searchClear');
    el.ipList = $('ipList');
    el.tabBar = $('tabBar');
    el.tabIndicator = $('tabIndicator');
    el.tabBadge = $('tabBadge');
    el.tagSelect = $('tagSelect');
    el.tagSelectTrigger = $('tagSelectTrigger');
    el.tagSelectValue = $('tagSelectValue');
    el.tagSelectMenu = $('tagSelectMenu');

    el.drawerOverlay = $('drawerOverlay');
    el.drawerIp = $('drawerIp');
    el.drawerBadge = $('drawerBadge');
    el.drawerClose = $('drawerClose');
    el.drawerBody = $('drawerBody');
    el.drawerFailed = $('drawerFailed');
    el.drawerSuccess = $('drawerSuccess');
    el.drawerUserCount = $('drawerUserCount');
    el.drawerFirst = $('drawerFirst');
    el.drawerLast = $('drawerLast');
    el.drawerUsers = $('drawerUsers');
    el.drawerReasons = $('drawerReasons');
    el.drawerLogs = $('drawerLogs');
    el.drawerLogsCount = $('drawerLogsCount');

    Chart.init();
    bindEvents();
    positionIndicator();
    renderTagMenu();
    runScan();
  }

  function bindEvents() {
    el.rangeControl.addEventListener('click', function (e) {
      var btn = e.target.closest('.seg-btn');
      if (!btn || state.loading) return;
      var val = btn.dataset.range;
      if (val === state.range) return;
      state.range = val;
      syncRangeButtons();
      runScan();
    });

    el.scanBtn.addEventListener('click', runScan);
    el.retryBtn.addEventListener('click', runScan);

    el.tabBar.addEventListener('click', function (e) {
      var btn = e.target.closest('.tab-btn');
      if (!btn) return;
      switchTab(btn.dataset.tab);
    });

    el.tagSelectTrigger.addEventListener('click', function (e) {
      e.stopPropagation();
      toggleTagMenu();
    });

    el.tagSelectMenu.addEventListener('click', function (e) {
      var opt = e.target.closest('.select-option');
      if (!opt) return;
      state.tagFilter = opt.dataset.tag || '';
      el.tagSelectValue.textContent = state.tagFilter || '全部标签';
      el.tagSelect.classList.remove('open');
      el.tagSelectMenu.hidden = true;
      renderTagMenu();
      renderList();
    });

    document.addEventListener('click', function (e) {
      if (!el.tagSelect.contains(e.target)) closeTagMenu();
    });

    window.addEventListener('resize', function () {
      positionIndicator();
    });

    el.searchInput.addEventListener('input', function () {
      state.query = el.searchInput.value;
      renderList();
    });

    el.searchInput.addEventListener('focus', function () {
      el.searchBox.classList.add('focused');
    });

    el.searchInput.addEventListener('blur', function () {
      el.searchBox.classList.remove('focused');
    });

    el.searchInput.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') {
        el.searchInput.value = '';
        state.query = '';
        renderList();
      }
    });

    el.searchClear.addEventListener('click', function () {
      el.searchInput.value = '';
      state.query = '';
      renderList();
      el.searchInput.focus();
    });

    el.ipList.addEventListener('click', function (e) {
      var row = e.target.closest('.ip-row');
      if (!row) return;
      var ip = row.dataset.ip;
      var item = (state.scanData && state.scanData.ip_list || []).filter(function (i) { return i.ip === ip; })[0];
      if (item) openDrawer(item);
    });

    el.drawerClose.addEventListener('click', closeDrawer);
    el.drawerOverlay.addEventListener('click', function (e) {
      if (e.target === el.drawerOverlay) closeDrawer();
    });
    document.addEventListener('keydown', function (e) {
      if (e.key !== 'Escape') return;
      if (!el.drawerOverlay.hidden) return closeDrawer();
      closeTagMenu();
    });
  }

  function syncRangeButtons() {
    var btns = el.rangeControl.querySelectorAll('.seg-btn');
    for (var i = 0; i < btns.length; i++) {
      btns[i].classList.toggle('active', btns[i].dataset.range === state.range);
      btns[i].disabled = state.loading;
    }
  }

  function switchTab(tab) {
    if (tab === state.tab) return;
    state.tab = tab;

    var btns = el.tabBar.querySelectorAll('.tab-btn');
    for (var i = 0; i < btns.length; i++) {
      btns[i].classList.toggle('active', btns[i].dataset.tab === tab);
    }

    var panels = document.querySelectorAll('.tab-panel');
    for (var j = 0; j < panels.length; j++) {
      panels[j].classList.toggle('active', panels[j].dataset.panel === tab);
    }

    positionIndicator();
    closeTagMenu();

    // 从隐藏状态切回概览时重绘一次图表，避免 0 宽容器下算出的布局残留
    if (tab === 'overview' && state.scanData) Chart.setData(state.scanData.buckets || []);
  }

  function positionIndicator() {
    var active = el.tabBar.querySelector('.tab-btn.active');
    if (!active || !el.tabIndicator) return;
    var barRect = el.tabBar.getBoundingClientRect();
    var rect = active.getBoundingClientRect();
    el.tabIndicator.style.width = rect.width + 'px';
    el.tabIndicator.style.transform = 'translateX(' + (rect.left - barRect.left - 3) + 'px)';
  }

  function toggleTagMenu() {
    if (el.tagSelectMenu.hidden) {
      el.tagSelect.classList.add('open');
      el.tagSelectMenu.hidden = false;
      renderTagMenu();
    } else {
      closeTagMenu();
    }
  }

  function closeTagMenu() {
    el.tagSelect.classList.remove('open');
    el.tagSelectMenu.hidden = true;
  }

  // 统计当前结果里每个标签的来源数，供下拉展示
  function tagCounts() {
    var list = (state.scanData && state.scanData.ip_list) || [];
    var map = {};
    var order = [];
    list.forEach(function (item) {
      var tags = item.risk_tags && item.risk_tags.length ? item.risk_tags : ['审计记录'];
      tags.forEach(function (t) {
        if (!(t in map)) { map[t] = 0; order.push(t); }
        map[t]++;
      });
    });
    return { map: map, order: order, total: list.length };
  }

  function checkIcon() {
    return '<svg class="select-check" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round">' +
      '<polyline points="20 6 9 17 4 12"></polyline></svg>';
  }

  function renderTagMenu() {
    var info = tagCounts();
    var html = '<div class="select-option ' + (state.tagFilter ? '' : 'selected') + '" data-tag="">' +
      '<span>全部标签</span>' +
      '<span class="select-option-count">' + info.total + '</span>' +
      '</div>';

    info.order.forEach(function (tag) {
      html += '<div class="select-option ' + (state.tagFilter === tag ? 'selected' : '') + '" data-tag="' + esc(tag) + '">' +
        '<span>' + esc(tag) + '</span>' +
        '<span class="select-option-count">' + info.map[tag] + '</span>' +
        '</div>';
    });

    el.tagSelectMenu.innerHTML = html;

    // 筛选标签若已不存在（重新扫描后），复位
    if (state.tagFilter && !(state.tagFilter in info.map)) {
      state.tagFilter = '';
      el.tagSelectValue.textContent = '全部标签';
    }
  }

  function setLoading(on) {
    state.loading = on;
    el.scanBtn.disabled = on;
    el.scanBtnText.textContent = on ? '扫描中' : '重新扫描';
    el.scanIcon.classList.toggle('rotating', on);
    syncRangeButtons();
  }

  function showError(msg) {
    state.error = msg || '';
    if (!msg) { el.errorBanner.hidden = true; return; }
    el.errorText.textContent = msg;
    el.errorBanner.hidden = false;
  }

  function runScan() {
    setLoading(true);
    showError('');

    API.probe()
      .then(function (probe) {
        if (!probe.available && !probe.permission) {
          showError(probe.message || '未检测到可读的 SSH 认证日志，权限不足或未开启 SSH 审计。');
          return null;
        }
        return API.scan(state.range);
      })
      .then(function (data) {
        if (!data) return;
        state.scanData = data;
        render();
      })
      .catch(function (err) {
        showError((err && err.message) || '扫描执行失败，请检查服务器日志状态。');
      })
      .then(function () {
        setLoading(false);
      });
  }

  function render() {
    var data = state.scanData;
    if (!data) return;

    var summary = data.summary || {};
    el.statFailed.textContent = summary.total_failed || 0;
    el.statSuccess.textContent = summary.total_success || 0;
    el.statIps.textContent = summary.unique_ips || 0;

    el.mockBadge.hidden = !API.isMock();

    Chart.setData(data.buckets || []);

    var ips = data.ip_list || [];
    el.tabBadge.hidden = ips.length === 0;
    el.tabBadge.textContent = ips.length;

    renderTagMenu();
    renderList();
    positionIndicator();
  }

  function filteredIps() {
    var list = (state.scanData && state.scanData.ip_list) || [];
    var q = state.query.trim().toLowerCase();
    var tag = state.tagFilter;

    if (!q && !tag) return list;

    return list.filter(function (item) {
      if (q && item.ip.toLowerCase().indexOf(q) === -1) return false;
      if (tag) {
        var tags = item.risk_tags || [];
        if (tags.indexOf(tag) === -1) return false;
      }
      return true;
    });
  }

  function userSummary(users) {
    var list = (users || []).filter(function (u) { return !!u && u.trim(); });
    if (!list.length) return '无';
    return list.slice(0, 4).join(', ') + (list.length > 4 ? '…' : '');
  }

  function esc(s) {
    return String(s == null ? '' : s)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
  }

  function renderList() {
    var list = filteredIps();
    var all = (state.scanData && state.scanData.ip_list) || [];
    var filtering = !!state.query.trim() || !!state.tagFilter;

    // 计数
    if (all.length) {
      el.sectionCount.textContent = (filtering ? list.length + ' / ' + all.length : all.length) + ' 个来源';
    } else {
      el.sectionCount.textContent = '';
    }

    el.searchClear.hidden = !state.query;

    // 空状态
    if (filtering && all.length && list.length === 0) {
      el.ipList.className = 'ip-list empty-list';
      el.ipList.innerHTML =
        '<div class="empty-state">' +
        '<svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">' +
        '<circle cx="11" cy="11" r="7"></circle><line x1="16.5" y1="16.5" x2="21" y2="21"></line></svg>' +
        '<p class="empty-title">' +
          (state.query.trim() ? '没有匹配 \'' + esc(state.query.trim()) + '\' 的来源' : '没有匹配「' + esc(state.tagFilter) + '」标签的来源') +
        '</p>' +
        '<span class="empty-desc">试试换个标签，或只输入 IP 的前几段，例如 212.18</span>' +
        '</div>';
      return;
    }

    if (all.length === 0) {
      el.ipList.className = 'ip-list empty-list';
      el.ipList.innerHTML =
        '<div class="empty-state">' +
        '<svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">' +
        '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path></svg>' +
        '<p class="empty-title">' + esc((state.scanData && state.scanData.empty_reason) || '在可读取的日志范围内，暂未发现失败认证记录。') + '</p>' +
        '<span class="empty-desc">未发现异常的高频或多账号登录尝试行为</span>' +
        '</div>';
      return;
    }

    el.ipList.className = 'ip-list';

    var html = list.map(function (item) {
      var tagText = (item.risk_tags && item.risk_tags[0]) || '审计记录';
      var failedBadge = item.failed_count > 0
        ? '<div class="stat-badge badge-failed"><span class="num">' + item.failed_count + '</span><span class="unit">次失败</span></div>'
        : '';
      var successBadge = item.success_count > 0
        ? '<div class="stat-badge badge-success"><span class="num">' + item.success_count + '</span><span class="unit">次成功</span></div>'
        : '';

      return '<div class="ip-row" data-ip="' + esc(item.ip) + '">' +
        '<div class="row-left">' +
          '<div class="ip-main">' +
            '<span class="ip-addr">' + esc(item.ip) + '</span>' +
            '<span class="risk-tag tag-' + esc(item.risk_level) + '">' + esc(tagText) + '</span>' +
          '</div>' +
          '<div class="ip-users">' +
            '<span class="users-label">尝试账号:</span>' +
            '<span class="users-val" title="' + esc((item.users || []).join(', ')) + '">' + esc(userSummary(item.users)) + '</span>' +
          '</div>' +
        '</div>' +
        '<div class="row-right">' + failedBadge + successBadge +
          '<div class="last-time">' + esc(item.last_seen) + '</div>' +
          '<svg class="chevron-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">' +
          '<polyline points="9 18 15 12 9 6"></polyline></svg>' +
        '</div>' +
      '</div>';
    }).join('');

    el.ipList.innerHTML = html;
  }

  function badgeClass(level) {
    if (level === 'high') return 'badge-high';
    if (level === 'medium') return 'badge-medium';
    if (level === 'low') return 'badge-low';
    return 'badge-info';
  }

  function openDrawer(item) {
    state.selectedIp = item;

    el.drawerIp.textContent = item.ip;
    el.drawerBadge.textContent = (item.risk_tags && item.risk_tags[0]) || '审计记录';
    el.drawerBadge.className = 'risk-badge ' + badgeClass(item.risk_level);

    el.drawerFailed.textContent = item.failed_count;
    el.drawerSuccess.textContent = item.success_count;
    el.drawerUserCount.textContent = (item.users || []).length;

    el.drawerFirst.textContent = item.first_seen;
    el.drawerLast.textContent = item.last_seen;

    el.drawerUsers.innerHTML = (item.users || []).map(function (u) {
      return '<span class="user-chip">' + esc(u) + '</span>';
    }).join('');

    el.drawerReasons.innerHTML = (item.risk_reasons || []).map(function (r) {
      return '<div class="reason-item"><span class="reason-dot"></span><span class="reason-text">' + esc(r) + '</span></div>';
    }).join('');

    var logs = item.logs || [];
    el.drawerLogsCount.textContent = '样本 ' + logs.length + ' 条 · 按时间升序最多 30 条';
    el.drawerLogs.innerHTML = logs.map(function (log, idx) {
      return '<div class="log-line"><span class="log-index">' + (idx + 1) + '</span><span class="log-text">' + esc(log) + '</span></div>';
    }).join('');

    el.drawerBody.scrollTop = 0;
    el.drawerOverlay.hidden = false;
  }

  function closeDrawer() {
    el.drawerOverlay.hidden = true;
    state.selectedIp = null;
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})(window);
