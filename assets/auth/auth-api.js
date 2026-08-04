// Shared helpers for the auth pages (login/register/forgot-password/reset-password):
// dark/light theme toggle and small wrappers around the JSON /auth API.
// Theme storage key matches the dashboard's own toggle (see layouts/dashboard-layout)
// so the preference carries over between the two.
(function () {
  var THEME_KEY = "darkMode";

  function applyTheme(isDark) {
    document.body.classList.toggle("dark", isDark);
    document.body.classList.toggle("bg-gray-900", isDark);
    var sun = document.getElementById("icon-sun");
    var moon = document.getElementById("icon-moon");
    if (sun && moon) {
      sun.classList.toggle("hidden", !isDark);
      moon.classList.toggle("hidden", isDark);
    }
  }

  function initTheme() {
    var isDark = false;
    try {
      var stored = localStorage.getItem(THEME_KEY);
      isDark = stored !== null
        ? JSON.parse(stored)
        : window.matchMedia("(prefers-color-scheme: dark)").matches;
    } catch (e) {}
    applyTheme(isDark);

    var toggle = document.getElementById("theme-toggle");
    if (toggle) {
      toggle.addEventListener("click", function () {
        var next = !document.body.classList.contains("dark");
        try { localStorage.setItem(THEME_KEY, JSON.stringify(next)); } catch (e) {}
        applyTheme(next);
      });
    }
  }

  async function getCsrfToken() {
    var res = await fetch("/auth/csrf-token", { credentials: "include" });
    var data = await res.json();
    return data.csrf_token;
  }

  async function postJSON(url, body) {
    var res = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    var data = {};
    try { data = await res.json(); } catch (e) {}
    return { ok: res.ok, status: res.status, data: data };
  }

  var alertVariants = {
    error: ["bg-red-50", "text-red-700", "border", "border-red-200", "dark:bg-red-900/30", "dark:text-red-300", "dark:border-red-800"],
    success: ["bg-green-50", "text-green-700", "border", "border-green-200", "dark:bg-green-900/30", "dark:text-green-300", "dark:border-green-800"],
  };

  function showAlert(el, message, variant) {
    if (!el) return;
    el.textContent = message;
    el.classList.remove("hidden");
    alertVariants.error.forEach(function (c) { el.classList.remove(c); });
    alertVariants.success.forEach(function (c) { el.classList.remove(c); });
    (alertVariants[variant] || alertVariants.error).forEach(function (c) { el.classList.add(c); });
  }

  function hideAlert(el) {
    if (el) el.classList.add("hidden");
  }

  function setLoading(button, loading, loadingText) {
    if (!button) return;
    if (loading) {
      button.dataset.originalText = button.dataset.originalText || button.textContent;
      button.disabled = true;
      button.classList.add("opacity-70", "cursor-not-allowed");
      button.textContent = loadingText || "Please wait…";
    } else {
      button.disabled = false;
      button.classList.remove("opacity-70", "cursor-not-allowed");
      button.textContent = button.dataset.originalText || button.textContent;
    }
  }

  function smoothRedirect(url, delay) {
    var card = document.getElementById("auth-card");
    if (card) card.classList.add("opacity-0", "scale-95");
    setTimeout(function () { window.location.href = url; }, delay || 400);
  }

  window.AuthAPI = {
    initTheme: initTheme,
    getCsrfToken: getCsrfToken,
    postJSON: postJSON,
    showAlert: showAlert,
    hideAlert: hideAlert,
    setLoading: setLoading,
    smoothRedirect: smoothRedirect,
  };

  document.addEventListener("DOMContentLoaded", initTheme);
})();
