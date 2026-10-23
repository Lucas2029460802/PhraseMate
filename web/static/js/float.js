(() => {
  const term = document.getElementById("term");
  const toastEl = document.getElementById("toast");
  const shell = document.querySelector(".shell");
  const btnNotebook = document.getElementById("btnNotebook");
  const dragArea = document.getElementById("dragArea");
  let busy = false;
  let queued = "";
  let appliedTheme = "";

  function canonicalTheme(theme) {
    if (theme === "warm" || theme === "beige") return "warm";
    if (theme === "pixel" || theme === "cute" || theme === "kawaii") return "pixel";
    if (theme === "tech" || theme === "neon" || theme === "cyber" || theme === "future") return "tech";
    return "mist";
  }

  function applyTheme(theme) {
    const t = canonicalTheme(theme);
    if (t === appliedTheme) return;
    appliedTheme = t;
    if (t === "mist") document.documentElement.removeAttribute("data-theme");
    else document.documentElement.setAttribute("data-theme", t);
  }

  async function syncTheme() {
    try {
      const res = await fetch("/api/settings");
      if (!res.ok) return;
      const data = await res.json();
      applyTheme(data.theme);
    } catch (_) {
      /* keep current */
    }
  }

  syncTheme();
  setInterval(syncTheme, 2000);
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) syncTheme();
  });

  function toast(msg, ok = true) {
    toastEl.textContent = msg;
    toastEl.hidden = false;
    toastEl.className = "toast show " + (ok ? "ok" : "bad");
    shell.classList.toggle("ok-flash", ok);
    shell.classList.toggle("bad-flash", !ok);
    clearTimeout(toast._t);
    toast._t = setTimeout(() => {
      toastEl.classList.remove("show");
      shell.classList.remove("ok-flash", "bad-flash");
      setTimeout(() => {
        toastEl.hidden = true;
      }, 200);
    }, 1200);
  }

  async function postCapture(value) {
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), 8000);
    try {
      const res = await fetch("/api/capture", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ term: value }),
        signal: ctrl.signal,
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(data.error || "收录失败");
      }
      return data.term || value;
    } catch (err) {
      if (err && err.name === "AbortError") {
        throw new Error("收录超时，请再试一次");
      }
      throw err;
    } finally {
      clearTimeout(timer);
    }
  }

  async function capture() {
    const value = (term.value || "").trim();
    if (!value) return;
    if (busy) {
      queued = value;
      term.value = "";
      return;
    }
    busy = true;
    term.value = "";
    let current = value;
    try {
      while (current) {
        const saved = await postCapture(current);
        toast("已收录", true);
        if (typeof window.notifyCaptured === "function") {
          window.notifyCaptured(saved);
        }
        current = queued;
        queued = "";
      }
    } catch (err) {
      if (current && !term.value) term.value = current;
      toast(err.message || "收录失败", false);
    } finally {
      busy = false;
      term.focus();
    }
  }

  term.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      capture();
    }
  });

  btnNotebook.addEventListener("click", () => {
    if (typeof window.toggleNotebook === "function") {
      window.toggleNotebook();
    }
  });

  // Drag via Go bridge (frameless window).
  const startDrag = (e) => {
    if (e.target.closest("input, button")) return;
    if (typeof window.startWindowDrag === "function") {
      window.startWindowDrag();
    }
  };
  dragArea.addEventListener("mousedown", startDrag);
  document.querySelector(".badge").addEventListener("mousedown", startDrag);

  term.focus();
})();
