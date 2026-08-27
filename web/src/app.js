// Progressive enhancement & real-time telemetry for Ninja Fintech Suite
(function () {
  "use strict";

  function showToast(text, kind) {
    if (!text) return;
    var isErr = /fail|error|invalid|insufficient|denied|unauthorized|cancel|reject|mismatch|unavailable|blocked|alert|hold|underage/i.test(text);
    if (!kind) {
      kind = isErr ? "err" : "ok";
    }
    var wrap = document.querySelector(".flash-wrap");
    if (!wrap) {
      wrap = document.createElement("div");
      wrap.className = "flash-wrap";
      document.body.appendChild(wrap);
    }

    var el = document.createElement("div");
    el.className = "flash " + kind;
    el.setAttribute("role", "alert");

    var iconBox = document.createElement("div");
    iconBox.className = "flash-icon";
    iconBox.innerHTML = kind === "err" ? "⛔" : "✓";

    var content = document.createElement("div");
    content.className = "flash-content";

    var title = document.createElement("strong");
    title.className = "flash-title";
    title.textContent = kind === "err" ? "Compliance Gate: Action Blocked" : "Compliance Verification Cleared";

    var msg = document.createElement("div");
    msg.className = "flash-message";
    msg.textContent = text;

    content.appendChild(title);
    content.appendChild(msg);

    var close = document.createElement("button");
    close.className = "flash-close";
    close.setAttribute("aria-label", "Dismiss notification");
    close.innerHTML = "&times;";
    close.onclick = function () {
      el.classList.add("flash-exit");
      setTimeout(function () { el.remove(); }, 200);
    };

    el.appendChild(iconBox);
    el.appendChild(content);
    el.appendChild(close);
    wrap.appendChild(el);

    setTimeout(function () {
      if (el.parentNode) {
        el.classList.add("flash-exit");
        setTimeout(function () { el.remove(); }, 250);
      }
    }, 7000);
  }

  function initFlash() {
    var flash = document.body.getAttribute("data-flash");
    if (flash) showToast(flash);
  }

  function initCopyButtons() {
    document.addEventListener("click", function (e) {
      var btn = e.target.closest("[data-copy]");
      if (!btn) return;
      e.preventDefault();
      var value = btn.getAttribute("data-copy");
      (navigator.clipboard && navigator.clipboard.writeText ? navigator.clipboard.writeText(value) : Promise.reject())
        .then(function () {
          var original = btn.textContent;
          btn.textContent = "Copied ✓";
          btn.classList.add("copied");
          setTimeout(function () {
            btn.textContent = original;
            btn.classList.remove("copied");
          }, 1500);
        })
        .catch(function () {
          window.prompt("Copy this value:", value);
        });
    });
  }

  function initConfirm() {
    document.addEventListener("submit", function (e) {
      var msg = e.target.getAttribute("data-confirm");
      if (msg && !window.confirm(msg)) e.preventDefault();
    });
  }

  function fillFields(map) {
    Object.keys(map).forEach(function (sel) {
      var el = document.querySelector(sel);
      if (!el) return;
      el.value = map[sel];
      el.dispatchEvent(new Event("input", { bubbles: true }));
      el.dispatchEvent(new Event("change", { bubbles: true }));
    });
  }

  function initPresetButtons() {
    document.addEventListener("click", function (e) {
      var btn = e.target.closest("[data-fill]");
      if (!btn) return;
      e.preventDefault();
      try {
        var map = JSON.parse(btn.getAttribute("data-fill"));
        fillFields(map);
        var submitTarget = btn.getAttribute("data-submit");
        if (submitTarget) {
          var form = document.querySelector(submitTarget);
          if (form) {
            form.submit();
          }
        }
      } catch (err) {
        console.error("preset fill error:", err);
      }
    });
  }

  // Live polling for the Activity Stream
  function initActivityStreamPolling() {
    var container = document.querySelector(".activity-list");
    if (!container) return;

    setInterval(function () {
      fetch("/api/activity-stream")
        .then(function (res) { return res.json(); })
        .then(function (logs) {
          if (!logs || !logs.length) return;
          var html = logs.map(function (log) {
            var statusClass = log.StatusCode === 200 ? "activity-status-200" : "activity-status-err";
            var modeBadge = log.IsMock 
              ? '<span style="font-size:.65rem; padding:1px 5px; border-radius:4px; background:var(--amber-bg); color:#fbbf24; margin-left:6px;">SIMULATED</span>'
              : '<span style="font-size:.65rem; padding:1px 5px; border-radius:4px; background:var(--green-bg); color:#34d399; margin-left:6px;">LIVE SANDBOX</span>';
            return '<div class="activity-row">' +
              '<div>' +
                '<span class="activity-method">' + log.Method + '</span>' +
                '<span style="color:#fff; margin-left:6px;">' + log.Endpoint + '</span>' +
                modeBadge +
              '</div>' +
              '<div style="display:flex; align-items:center; gap:12px;">' +
                '<span class="' + statusClass + '">HTTP ' + log.StatusCode + '</span>' +
                '<span class="activity-duration">' + log.DurationMs + 'ms</span>' +
              '</div>' +
            '</div>';
          }).join("");
          container.innerHTML = html;
        })
        .catch(function () {});
    }, 4000);
  }

  document.addEventListener("DOMContentLoaded", function () {
    initFlash();
    initCopyButtons();
    initConfirm();
    initPresetButtons();
    initActivityStreamPolling();
  });

  window.NinjaDemo = { showToast: showToast, fillFields: fillFields };
})();
