(() => {
  const term = document.getElementById("term");
  const toastEl = document.getElementById("toast");
  const shell = document.querySelector(".shell");
  const btnNotebook = document.getElementById("btnNotebook");
  const dragArea = document.getElementById("dragArea");
  let busy = false;

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

  async function capture() {
    if (busy) return;
    const value = (term.value || "").trim();
    if (!value) return;
    busy = true;
    term.disabled = true;
    try {
      const res = await fetch("/api/capture", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ term: value }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(data.error || "收录失败");
      }
      term.value = "";
      toast("已收录", true);
      if (typeof window.notifyCaptured === "function") {
        window.notifyCaptured(data.term || value);
      }
    } catch (err) {
      toast(err.message || "收录失败", false);
    } finally {
      busy = false;
      term.disabled = false;
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
