(() => {
  const $ = (sel) => document.querySelector(sel);

  const els = {
    statusPill: $("#statusPill"),
    wordList: $("#wordList"),
    wordDetail: $("#wordDetail"),
    wordCount: $("#wordCount"),
    filterInput: $("#filterInput"),
    btnNewQuiz: $("#btnNewQuiz"),
    btnGoQuiz: $("#btnGoQuiz"),
    btnBackNotebook: $("#btnBackNotebook"),
    btnShowFloat: $("#btnShowFloat"),
    btnSettings: $("#btnSettings"),
    settingsModal: $("#settingsModal"),
    btnCloseSettings: $("#btnCloseSettings"),
    settingsForm: $("#settingsForm"),
    settingsApiKey: $("#settingsApiKey"),
    settingsBaseURL: $("#settingsBaseURL"),
    settingsModel: $("#settingsModel"),
    settingsKeyHint: $("#settingsKeyHint"),
    btnClearKey: $("#btnClearKey"),
    btnSaveSettings: $("#btnSaveSettings"),
    quizArea: $("#quizArea"),
    pageNotebook: $("#pageNotebook"),
    pageQuiz: $("#pageQuiz"),
    toast: $("#toast"),
  };

  let words = [];
  let selectedWordId = null;
  let listFingerprint = "";
  let statusFingerprint = "";
  let quizScore = { correct: 0, total: 0 };
  let pendingTimer = null;

  async function api(path, options = {}) {
    const res = await fetch(path, {
      headers: { "Content-Type": "application/json", ...(options.headers || {}) },
      ...options,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(data.error || `请求失败 (${res.status})`);
    }
    return data;
  }

  function toast(msg) {
    els.toast.textContent = msg;
    els.toast.hidden = false;
    requestAnimationFrame(() => els.toast.classList.add("show"));
    clearTimeout(toast._t);
    toast._t = setTimeout(() => {
      els.toast.classList.remove("show");
      setTimeout(() => {
        els.toast.hidden = true;
      }, 250);
    }, 2200);
  }

  function escapeHtml(s) {
    return String(s ?? "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function fpWords(list) {
    return list
      .map(
        (w) =>
          `${w.id}|${w.status}|${w.meaning_zh}|${w.meaning_en}|${w.error_msg || ""}|${w.created_at}`
      )
      .join(";");
  }

  function showPage(name) {
    const isQuiz = name === "quiz";
    els.pageNotebook.hidden = isQuiz;
    els.pageQuiz.hidden = !isQuiz;
    document.body.classList.toggle("on-quiz", isQuiz);
  }

  let voicesCache = null;

  function loadVoices() {
    if (!window.speechSynthesis) return [];
    const list = window.speechSynthesis.getVoices();
    if (list.length) voicesCache = list;
    return voicesCache || list;
  }

  if (window.speechSynthesis) {
    loadVoices();
    window.speechSynthesis.onvoiceschanged = loadVoices;
  }

  function pickEnglishVoice() {
    const voices = loadVoices();
    return (
      voices.find((v) => v.lang === "en-US") ||
      voices.find((v) => v.lang.startsWith("en-GB")) ||
      voices.find((v) => v.lang.startsWith("en")) ||
      null
    );
  }

  function speakEnglish(text) {
    const content = String(text || "").trim();
    if (!content) return;
    if (!window.speechSynthesis) {
      toast("当前环境不支持语音朗读");
      return;
    }
    window.speechSynthesis.cancel();
    const utter = new SpeechSynthesisUtterance(content);
    utter.lang = "en-US";
    const voice = pickEnglishVoice();
    if (voice) utter.voice = voice;
    utter.pitch = 1;
    utter.onerror = () => toast("朗读失败，请检查系统语音设置");
    window.speechSynthesis.speak(utter);
  }

  function statusLabel(w) {
    if (w.status === "pending") return "生成中";
    if (w.status === "error") return "失败";
    return "";
  }

  function renderMeta(w) {
    const pos = (w.part_of_speech || "").trim();
    const phonetic = (w.phonetic || "").trim();
    if (pos && phonetic) {
      return `${escapeHtml(pos)} <span class="phonetic">${escapeHtml(phonetic)}</span>`;
    }
    if (phonetic) {
      return `<span class="phonetic">${escapeHtml(phonetic)}</span>`;
    }
    return escapeHtml(pos);
  }

  function renderListRow(w) {
    const meta = renderMeta(w);
    const status = statusLabel(w);
    const active = selectedWordId === w.id ? " active" : "";
    return `
      <button type="button" class="word-row${active}" data-id="${w.id}">
        <span class="word-row-main">
          <span class="term">${escapeHtml(w.term)}</span>
          ${meta ? `<span class="meta">${meta}</span>` : ""}
        </span>
        ${status ? `<span class="word-status ${w.status}">${status}</span>` : ""}
      </button>`;
  }

  function renderDetailHTML(w) {
    const meta = renderMeta(w);
    let body = "";
    if (w.status === "pending") {
      body = `<p class="meaning-zh pending">释义生成中…</p>`;
    } else if (w.status === "error") {
      body = `<p class="meaning-zh error">释义失败：${escapeHtml(w.error_msg || "未知错误")}</p>
        <div class="word-actions"><button type="button" class="btn ghost sm" data-retry="${w.id}">重试</button></div>`;
    } else {
      const example =
        w.example_en
          ? `<p class="example">${escapeHtml(w.example_en)}${
              w.example_zh ? `<br />${escapeHtml(w.example_zh)}` : ""
            }<button type="button" class="btn ghost sm speak-inline" data-speak="${escapeHtml(
              w.example_en
            )}" title="朗读例句">朗读例句</button></p>`
          : "";
      const meaningZh = (w.meaning_zh || "").trim();
      body = `
        ${meaningZh ? `<p class="meaning-zh">${escapeHtml(meaningZh)}</p>` : ""}
        <p class="meaning-en">${escapeHtml(w.meaning_en)}</p>
        ${example}`;
    }
    return `
      <div class="detail-head">
        <div>
          <div class="term-row">
            <h3 class="term">${escapeHtml(w.term)}</h3>
            <button type="button" class="btn-speak" data-speak="${escapeHtml(w.term)}" title="朗读单词" aria-label="朗读单词">
              <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path fill="currentColor" d="M3 10v4h4l5 5V5L7 10H3zm13.5 2a4.5 4.5 0 0 0-2.1-3.8v7.6a4.48 4.48 0 0 0 2.1-3.8zM14 3.23v2.06a7 7 0 0 1 0 13.74v2.06a9 9 0 0 0 0-17.82z"/></svg>
            </button>
          </div>
          <p class="meta">${meta}</p>
        </div>
        <button type="button" class="btn danger sm" data-del="${w.id}">删除</button>
      </div>
      <div class="detail-body">${body}</div>`;
  }

  function filteredWords() {
    const q = (els.filterInput.value || "").trim().toLowerCase();
    if (!q) return words;
    return words.filter(
      (w) =>
        w.term.toLowerCase().includes(q) ||
        (w.meaning_zh || "").toLowerCase().includes(q) ||
        (w.meaning_en || "").toLowerCase().includes(q)
    );
  }

  function renderDetail() {
    const w = words.find((x) => x.id === selectedWordId);
    if (!w) {
      els.wordDetail.innerHTML = `<p class="empty">点击左侧单词查看释义</p>`;
      return;
    }
    els.wordDetail.innerHTML = renderDetailHTML(w);
  }

  function renderList() {
    const list = filteredWords();
    els.wordCount.textContent = String(words.length);

    if (selectedWordId && !words.some((w) => w.id === selectedWordId)) {
      selectedWordId = null;
    }

    if (!list.length) {
      els.wordList.innerHTML = `<p class="empty">${
        words.length
          ? "没有匹配的生词。"
          : "还没有生词。用右下角置顶速记条输入，回车即可立刻收录。"
      }</p>`;
      renderDetail();
      return;
    }

    if (!selectedWordId || !list.some((w) => w.id === selectedWordId)) {
      selectedWordId = list[0].id;
    }

    els.wordList.innerHTML = list.map(renderListRow).join("");
    renderDetail();
  }

  function selectWord(id) {
    if (selectedWordId === id) return;
    selectedWordId = id;
    els.wordList.querySelectorAll(".word-row").forEach((row) => {
      row.classList.toggle("active", Number(row.dataset.id) === id);
    });
    renderDetail();
  }

  function schedulePendingPoll() {
    const hasPending = words.some((w) => w.status === "pending");
    if (!hasPending) {
      if (pendingTimer) {
        clearInterval(pendingTimer);
        pendingTimer = null;
      }
      return;
    }
    if (pendingTimer) return;
    pendingTimer = setInterval(() => {
      loadWords({ silent: true }).catch(() => {});
    }, 1500);
  }

  async function loadStatus({ silent } = {}) {
    try {
      const s = await api("/api/status");
      const next = `${s.has_key}|${s.model}|${s.word_count}|${s.pending}|${s.data_branch || ""}|${s.sync_error || ""}`;
      if (silent && next === statusFingerprint) return;
      statusFingerprint = next;
      const syncNote = s.sync_error ? " · 生词同步失败" : "";
      if (s.has_key) {
        const pending = s.pending > 0 ? ` · ${s.pending} 生成中` : "";
        els.statusPill.textContent = `就绪 · ${s.model}${pending}${syncNote}`;
        els.statusPill.className = s.sync_error ? "status-pill warn" : "status-pill ok";
        els.statusPill.title = s.sync_error || (s.data_branch ? `生词本保存在 ${s.data_branch} 分支` : "点击打开设置");
      } else {
        els.statusPill.textContent = `未配置 API Key · 点击设置${syncNote}`;
        els.statusPill.className = "status-pill warn";
        els.statusPill.title = s.sync_error || "点击填写 API Key";
      }
    } catch {
      if (!silent) {
        els.statusPill.textContent = "服务未连接";
        els.statusPill.className = "status-pill warn";
      }
    }
  }

  const presets = {
    openai: {
      base_url: "https://api.openai.com/v1",
      model: "gpt-4o-mini",
    },
    deepseek: {
      base_url: "https://api.deepseek.com/v1",
      model: "deepseek-chat",
    },
  };

  async function openSettings() {
    els.settingsModal.hidden = false;
    els.settingsApiKey.value = "";
    try {
      const s = await api("/api/settings");
      els.settingsBaseURL.value = s.base_url || "";
      els.settingsModel.value = s.model || "";
      if (s.has_key) {
        els.settingsApiKey.placeholder = s.api_key_masked || "已保存，留空则不变";
        els.settingsKeyHint.textContent = `当前已保存：${s.api_key_masked || "••••"}（留空则保持不变）`;
      } else {
        els.settingsApiKey.placeholder = "sk-…";
        els.settingsKeyHint.textContent = "必填：OpenAI 兼容接口的 API Key";
      }
    } catch (err) {
      toast(err.message);
    }
    els.settingsApiKey.focus();
  }

  function closeSettings() {
    els.settingsModal.hidden = true;
  }

  async function saveSettings(clearKey = false) {
    const body = {
      api_key: (els.settingsApiKey.value || "").trim(),
      base_url: (els.settingsBaseURL.value || "").trim(),
      model: (els.settingsModel.value || "").trim(),
      clear_key: !!clearKey,
    };
    els.btnSaveSettings.disabled = true;
    try {
      await api("/api/settings", {
        method: "PUT",
        body: JSON.stringify(body),
      });
      toast(clearKey ? "已清除 API Key" : "设置已保存");
      closeSettings();
      await loadStatus();
    } catch (err) {
      toast(err.message);
    } finally {
      els.btnSaveSettings.disabled = false;
    }
  }

  async function loadWords({ silent } = {}) {
    const list = await api("/api/words");
    const next = fpWords(list);
    if (silent && next === listFingerprint) {
      schedulePendingPoll();
      if (selectedWordId) renderDetail();
      return;
    }
    listFingerprint = next;
    words = list;
    renderList();
    schedulePendingPoll();
  }

  async function refreshAll({ silent } = {}) {
    await Promise.all([loadWords({ silent }), loadStatus({ silent })]);
  }

  window.phrasemateRefresh = () => {
    refreshAll({ silent: false }).catch(() => {});
  };

  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) {
      refreshAll({ silent: true }).catch(() => {});
    }
  });
  window.addEventListener("focus", () => {
    refreshAll({ silent: true }).catch(() => {});
  });

  async function deleteWord(id) {
    if (!confirm("确定从生词本删除？")) return;
    await api(`/api/words/${id}`, { method: "DELETE" });
    toast("已删除");
    if (selectedWordId === id) selectedWordId = null;
    await refreshAll();
  }

  async function retryWord(id) {
    await api(`/api/words/${id}/retry`, { method: "POST" });
    toast("已重新排队生成释义");
    await refreshAll();
  }

  async function createQuiz() {
    els.btnNewQuiz.disabled = true;
    els.quizArea.innerHTML = `<p class="empty">Generating English quiz…</p>`;
    quizScore = { correct: 0, total: 0 };
    try {
      const data = await api("/api/quiz", {
        method: "POST",
        body: JSON.stringify({ count: 5 }),
      });
      const qs = data.questions || [];
      if (!qs.length) {
        els.quizArea.innerHTML = `<p class="empty">No questions generated. Try again.</p>`;
        return;
      }
      quizScore.total = qs.length;
      els.quizArea.innerHTML = `
        <div class="score-banner" id="scoreBanner">Progress: 0 / ${qs.length}</div>
        ${qs
          .map(
            (q, qi) => `
          <div class="quiz-q" data-qi="${qi}">
            <h3>${qi + 1}. ${escapeHtml(q.question)}</h3>
            <div class="options">
              ${(q.options || [])
                .map(
                  (opt, oi) =>
                    `<button type="button" class="opt" data-qi="${qi}" data-oi="${oi}" data-correct="${q.correct_index}" data-explain="${escapeHtml(
                      q.explain_answer || ""
                    )}">${escapeHtml(opt)}</button>`
                )
                .join("")}
            </div>
            <p class="explain" id="explain-${qi}" hidden></p>
          </div>`
          )
          .join("")}
      `;
    } catch (err) {
      els.quizArea.innerHTML = `<p class="empty">${escapeHtml(err.message)}</p>`;
      toast(err.message);
    } finally {
      els.btnNewQuiz.disabled = false;
    }
  }

  function updateScoreBanner() {
    const el = $("#scoreBanner");
    if (!el) return;
    const done = els.quizArea.querySelectorAll(".quiz-q.answered").length;
    el.textContent =
      done >= quizScore.total
        ? `Done! Score ${quizScore.correct} / ${quizScore.total}`
        : `Progress: ${done} / ${quizScore.total} · Correct ${quizScore.correct}`;
  }

  els.filterInput.addEventListener("input", renderList);

  els.wordList.addEventListener("click", (e) => {
    const row = e.target.closest(".word-row");
    if (row) {
      selectWord(Number(row.dataset.id));
    }
  });

  els.wordDetail.addEventListener("click", (e) => {
    const speak = e.target.closest("[data-speak]");
    if (speak) {
      speakEnglish(speak.dataset.speak);
      return;
    }
    const del = e.target.closest("[data-del]");
    if (del) {
      deleteWord(Number(del.dataset.del));
      return;
    }
    const retry = e.target.closest("[data-retry]");
    if (retry) {
      retryWord(Number(retry.dataset.retry));
    }
  });

  els.btnGoQuiz.addEventListener("click", () => {
    showPage("quiz");
  });

  els.btnBackNotebook.addEventListener("click", () => {
    showPage("notebook");
  });

  els.btnNewQuiz.addEventListener("click", createQuiz);

  els.btnShowFloat?.addEventListener("click", () => {
    if (typeof window.showFloatWindow === "function") {
      window.showFloatWindow();
    } else {
      toast("请在桌面模式使用系统速记窗");
    }
  });

  els.btnSettings?.addEventListener("click", () => {
    openSettings().catch((err) => toast(err.message));
  });

  els.statusPill?.addEventListener("click", () => {
    openSettings().catch((err) => toast(err.message));
  });

  els.btnCloseSettings?.addEventListener("click", closeSettings);

  els.settingsModal?.addEventListener("click", (e) => {
    if (e.target === els.settingsModal) closeSettings();
  });

  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && els.settingsModal && !els.settingsModal.hidden) {
      closeSettings();
    }
  });

  els.settingsForm?.addEventListener("submit", (e) => {
    e.preventDefault();
    saveSettings(false);
  });

  els.btnClearKey?.addEventListener("click", () => {
    if (!confirm("确定清除已保存的 API Key？")) return;
    saveSettings(true);
  });

  els.settingsForm?.querySelectorAll("[data-preset]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const p = presets[btn.dataset.preset];
      if (!p) return;
      els.settingsBaseURL.value = p.base_url;
      els.settingsModel.value = p.model;
    });
  });

  els.quizArea.addEventListener("click", (e) => {
    const btn = e.target.closest(".opt");
    if (!btn || btn.disabled) return;
    const qi = btn.dataset.qi;
    const block = els.quizArea.querySelector(`.quiz-q[data-qi="${qi}"]`);
    if (!block || block.classList.contains("answered")) return;

    const correct = Number(btn.dataset.correct);
    const chosen = Number(btn.dataset.oi);
    const options = block.querySelectorAll(".opt");
    options.forEach((o) => {
      o.disabled = true;
      if (Number(o.dataset.oi) === correct) o.classList.add("correct");
    });
    if (chosen === correct) {
      quizScore.correct += 1;
    } else {
      btn.classList.add("wrong");
    }
    block.classList.add("answered");
    const explain = $(`#explain-${qi}`);
    if (explain && btn.dataset.explain) {
      explain.hidden = false;
      explain.textContent = btn.dataset.explain;
    }
    updateScoreBanner();
  });

  showPage("notebook");
  loadStatus();
  loadWords().catch((err) => toast(err.message));
})();
