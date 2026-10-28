/* 红绿双色登录活动柱状图（纯 DOM + SVG，无依赖） */
(function (global) {
  'use strict';

  var SVG_W = 1000;
  var SVG_H = 180;
  var TOP = 20;
  var BOTTOM = 150;
  var CHART_H = BOTTOM - TOP;

  var el = {};
  var buckets = [];
  var groups = [];
  var hoverIndex = null;

  function init() {
    el.svg = document.getElementById('chartSvg');
    el.layer = document.getElementById('barsLayer');
    el.labels = document.getElementById('axisLabels');
    el.tooltip = document.getElementById('chartTooltip');
    el.ttTime = document.getElementById('ttTime');
    el.ttFailed = document.getElementById('ttFailed');
    el.ttSuccess = document.getElementById('ttSuccess');
  }

  function maxValue() {
    var m = 0;
    buckets.forEach(function (b) {
      if (b.failed_count > m) m = b.failed_count;
      if (b.success_count > m) m = b.success_count;
    });
    if (m === 0) return 10;
    return Math.ceil(m * 1.15);
  }

  function build() {
    groups = [];
    var n = buckets.length;
    if (!n) return;

    var maxVal = maxValue();
    var groupWidth = SVG_W / n;
    var barWidth = Math.max(4, Math.min(8, (groupWidth - 8) / 2));
    var gap = 2;

    buckets.forEach(function (b, i) {
      var centerX = i * groupWidth + groupWidth / 2;
      var failH = maxVal > 0 ? (b.failed_count / maxVal) * CHART_H : 0;
      var succH = maxVal > 0 ? (b.success_count / maxVal) * CHART_H : 0;

      groups.push({
        bucket: b,
        index: i,
        centerX: centerX,
        groupWidth: groupWidth,
        failX: centerX - barWidth - gap / 2,
        failY: BOTTOM - failH,
        failH: failH,
        succX: centerX + gap / 2,
        succY: BOTTOM - succH,
        succH: succH,
        barWidth: barWidth,
      });
    });
  }

  function rect(attrs, cls) {
    var r = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    Object.keys(attrs).forEach(function (k) { r.setAttribute(k, attrs[k]); });
    if (cls) r.setAttribute('class', cls);
    return r;
  }

  function render() {
    el.layer.innerHTML = '';
    var frag = document.createDocumentFragment();

    groups.forEach(function (g) {
      var group = document.createElementNS('http://www.w3.org/2000/svg', 'g');
      group.setAttribute('class', 'bar-group');
      group.dataset.index = String(g.index);

      // 触发热区
      group.appendChild(rect({
        x: g.centerX - g.groupWidth / 2,
        y: TOP,
        width: g.groupWidth,
        height: CHART_H + 10,
        fill: 'transparent',
      }, 'hover-trigger'));

      if (g.failH > 0) {
        group.appendChild(rect({
          x: g.failX, y: g.failY, width: g.barWidth, height: g.failH, rx: 2.5, ry: 2.5,
        }, 'bar bar-failed'));
      }

      if (g.succH > 0) {
        group.appendChild(rect({
          x: g.succX, y: g.succY, width: g.barWidth, height: g.succH, rx: 2.5, ry: 2.5,
        }, 'bar bar-success'));
      }

      group.addEventListener('mouseenter', function () { setHover(g.index); });
      group.addEventListener('mouseleave', function () { setHover(null); });

      frag.appendChild(group);
    });

    el.layer.appendChild(frag);
    renderLabels();
  }

  function renderLabels() {
    el.labels.innerHTML = '';
    var n = groups.length;
    if (!n) return;

    var step = n <= 24 ? 4 : n <= 28 ? 4 : 5;

    groups.forEach(function (g, i) {
      var isFirst = i === 0;
      var isLast = i === n - 1;
      if (!(isFirst || isLast || i % step === 0)) return;

      var span = document.createElement('span');
      span.className = 'axis-label anchor-' + (isFirst ? 'left' : isLast ? 'right' : 'center');
      span.style.left = ((g.centerX / SVG_W) * 100) + '%';
      span.textContent = g.bucket.label;
      el.labels.appendChild(span);
    });
  }

  function setHover(index) {
    hoverIndex = index;

    var nodes = el.layer.querySelectorAll('.bar-group');
    for (var i = 0; i < nodes.length; i++) {
      var node = nodes[i];
      var isActive = Number(node.dataset.index) === index;
      node.classList.toggle('active', index !== null && isActive);
      node.classList.toggle('dim', index !== null && !isActive);
    }

    if (index === null) {
      el.tooltip.hidden = true;
      return;
    }

    var g = groups[index];
    if (!g) { el.tooltip.hidden = true; return; }

    el.ttTime.textContent = g.bucket.label;
    el.ttFailed.textContent = g.bucket.failed_count;
    el.ttSuccess.textContent = g.bucket.success_count;

    var pct = (g.centerX / SVG_W) * 100;
    el.tooltip.style.left = 'clamp(60px, ' + pct + '%, calc(100% - 130px))';
    el.tooltip.style.top = '25px';
    el.tooltip.hidden = false;
  }

  global.Chart = {
    init: init,
    setData: function (list) {
      buckets = list || [];
      hoverIndex = null;
      el.tooltip.hidden = true;
      build();
      render();
    },
  };
})(window);
