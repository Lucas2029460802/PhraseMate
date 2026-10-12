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
    btnGoWriting: $("#btnGoWriting"),
    btnBackNotebook: $("#btnBackNotebook"),
    btnBackFromWriting: $("#btnBackFromWriting"),
    btnNewWriting: $("#btnNewWriting"),
    btnDeleteWriting: $("#btnDeleteWriting"),
    btnUploadModel: $("#btnUploadModel"),
    writingList: $("#writingList"),
    writingCount: $("#writingCount"),
    writingEditor: $("#writingEditor"),
    writingEmpty: $("#writingEmpty"),
    writingPrompt: $("#writingPrompt"),
    writingPromptField: $("#writingPromptField"),
    writingPromptImages: $("#writingPromptImages"),
    writingPromptImageFile: $("#writingPromptImageFile"),
    btnAddPromptImage: $("#btnAddPromptImage"),
    writingModel: $("#writingModel"),
    writingModelPreview: $("#writingModelPreview"),
    writingModelRendered: $("#writingModelRendered"),
    writingModelFile: $("#writingModelFile"),
    writingModelFilename: $("#writingModelFilename"),
    writingSaveHint: $("#writingSaveHint"),
    btnToggleModelEdit: $("#btnToggleModelEdit"),
    btnExpandModel: $("#btnExpandModel"),
    writingReader: $("#writingReader"),
    writingReaderBody: $("#writingReaderBody"),
    writingReaderZoom: $("#writingReaderZoom"),
    writingZoomHint: $("#writingZoomHint"),
    btnCloseWritingReader: $("#btnCloseWritingReader"),
    writingImageLightbox: $("#writingImageLightbox"),
    writingLightboxImg: $("#writingLightboxImg"),
    btnCloseImageLightbox: $("#btnCloseImageLightbox"),
    btnToggleTranslate: $("#btnToggleTranslate"),
    translatePanel: $("#translatePanel"),
    translateDirection: $("#translateDirection"),
    translateInput: $("#translateInput"),
    translateOutput: $("#translateOutput"),
    translateMeta: $("#translateMeta"),
    btnDoTranslate: $("#btnDoTranslate"),
    btnSwapTranslate: $("#btnSwapTranslate"),
    btnCopyTranslation: $("#btnCopyTranslation"),
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
    pageWriting: $("#pageWriting"),
    toast: $("#toast"),
  };

  let words = [];
  let selectedWordId = null;
  let listFingerprint = "";
  let statusFingerprint = "";
  let quizScore = { correct: 0, total: 0 };
  let pendingTimer = null;
  let writings = [];
  let selectedWritingId = null;
  let writingSaveTimer = null;
  let writingDirty = false;
  let writingSaving = false;
  let writingModelEditing = false;
  let promptImages = [];
  const MAX_PROMPT_IMAGES = 6;
  const bootPage = new URLSearchParams(location.search).get("page") || "notebook";
  const isAuxWritingWindow = bootPage === "writing";

  function openWritingWindowPreferred() {
    if (typeof window.openWritingWindow === "function") {
      window.openWritingWindow();
      return true;
    }
    try {
      window.open(`${location.origin}/?page=writing`, "phrasemate-writing");
      return true;
    } catch {
      return false;
    }
  }

  function showNotebookWindowPreferred() {
    if (typeof window.showNotebookWindow === "function") {
      window.showNotebookWindow();
      return true;
    }
    return false;
  }

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
          `${w.id}|${w.status}|${w.meaning_zh}|${w.meaning_en}|${w.error_msg || ""}|${w.created_at}|${extraFP(w)}`
      )
      .join(";");
  }

  function showPage(name) {
    const isQuiz = name === "quiz";
    const isWriting = name === "writing";
    els.pageNotebook.hidden = isQuiz || isWriting;
    els.pageQuiz.hidden = !isQuiz;
    if (els.pageWriting) els.pageWriting.hidden = !isWriting;
    document.body.classList.toggle("on-quiz", isQuiz);
    document.body.classList.toggle("on-writing", isWriting);
  }

  function formatWritingTime(iso) {
    if (!iso) return "";
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return "";
    return d.toLocaleString("zh-CN", {
      month: "numeric",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function setWritingSaveHint(text) {
    if (els.writingSaveHint) els.writingSaveHint.textContent = text || "";
  }

  function renderModelMarkdown(src) {
    const md = window.phrasemateMarkdown;
    const html = md ? md.render(src) : "";
    if (!html) {
      return `<p class="empty">粘贴或导入 Markdown 范文后，点击此处放大阅读。</p>`;
    }
    return html;
  }

  function refreshModelPreview() {
    if (!els.writingModelRendered) return;
    els.writingModelRendered.innerHTML = renderModelMarkdown(els.writingModel?.value || "");
  }

  function setModelEditMode(editing) {
    writingModelEditing = !!editing;
    if (els.writingModel) els.writingModel.hidden = !writingModelEditing;
    if (els.writingModelPreview) els.writingModelPreview.hidden = writingModelEditing;
    if (els.btnToggleModelEdit) {
      els.btnToggleModelEdit.textContent = writingModelEditing ? "完成编辑" : "编辑 Markdown";
    }
    if (els.btnExpandModel) els.btnExpandModel.disabled = writingModelEditing;
    if (!writingModelEditing) refreshModelPreview();
  }

  function imageDataURL(img) {
    if (!img?.data) return "";
    if (String(img.data).startsWith("data:")) return img.data;
    return `data:${img.mime || "image/png"};base64,${img.data}`;
  }

  function renderPromptImages() {
    if (!els.writingPromptImages) return;
    if (!promptImages.length) {
      els.writingPromptImages.hidden = true;
      els.writingPromptImages.innerHTML = "";
      return;
    }
    els.writingPromptImages.hidden = false;
    els.writingPromptImages.innerHTML = promptImages
      .map((img, i) => {
        const src = imageDataURL(img);
        return `<div class="writing-prompt-thumb">
          <img src="${src}" alt="题目截图 ${i + 1}" />
          <button type="button" class="thumb-remove" data-remove-image="${i}" aria-label="删除截图">×</button>
        </div>`;
      })
      .join("");
  }

  function openImageLightbox(src) {
    if (!src || !els.writingImageLightbox || !els.writingLightboxImg) return;
    els.writingLightboxImg.src = src;
    els.writingImageLightbox.hidden = false;
    requestAnimationFrame(() => {
      const box = els.writingImageLightbox;
      const img = els.writingLightboxImg;
      if (!box || !img) return;
      const x = Math.max(0, (img.offsetWidth - box.clientWidth) / 2);
      const y = Math.max(0, (img.offsetHeight - box.clientHeight) / 2);
      box.scrollLeft = x;
      box.scrollTop = y;
    });
  }

  function closeImageLightbox() {
    if (els.writingImageLightbox) {
      els.writingImageLightbox.hidden = true;
      els.writingImageLightbox.classList.remove("is-dragging");
    }
    if (els.writingLightboxImg) els.writingLightboxImg.removeAttribute("src");
  }

  function compressImageFile(file) {
    return new Promise((resolve, reject) => {
      if (!file || !String(file.type || "").startsWith("image/")) {
        reject(new Error("请粘贴或选择图片文件"));
        return;
      }
      const reader = new FileReader();
      reader.onerror = () => reject(new Error("读取图片失败"));
      reader.onload = () => {
        const img = new Image();
        img.onload = () => {
          const maxW = 1600;
          const scale = img.width > maxW ? maxW / img.width : 1;
          const w = Math.max(1, Math.round(img.width * scale));
          const h = Math.max(1, Math.round(img.height * scale));
          const canvas = document.createElement("canvas");
          canvas.width = w;
          canvas.height = h;
          const ctx = canvas.getContext("2d");
          ctx.drawImage(img, 0, 0, w, h);
          const preferPng = file.type === "image/png" || file.type === "image/gif";
          const mime = preferPng ? "image/png" : "image/jpeg";
          const dataURL = preferPng
            ? canvas.toDataURL("image/png")
            : canvas.toDataURL("image/jpeg", 0.86);
          const comma = dataURL.indexOf(",");
          resolve({
            mime,
            data: comma >= 0 ? dataURL.slice(comma + 1) : dataURL,
          });
        };
        img.onerror = () => reject(new Error("图片无法解析"));
        img.src = reader.result;
      };
      reader.readAsDataURL(file);
    });
  }

  async function addPromptImageFiles(files) {
    const list = Array.from(files || []).filter((f) => String(f.type || "").startsWith("image/"));
    if (!list.length) {
      toast("剪贴板里没有图片");
      return;
    }
    for (const file of list) {
      if (promptImages.length >= MAX_PROMPT_IMAGES) {
        toast(`最多添加 ${MAX_PROMPT_IMAGES} 张截图`);
        break;
      }
      try {
        const img = await compressImageFile(file);
        promptImages.push(img);
      } catch (err) {
        toast(err.message);
      }
    }
    renderPromptImages();
    scheduleWritingSave();
  }

  let readerZoom = 1;
  const READER_ZOOM_MIN = 0.8;
  const READER_ZOOM_MAX = 3.5;

  function setReaderZoom(next, anchor) {
    const scroller = els.writingReaderBody;
    const zoomEl = els.writingReaderZoom;
    if (!scroller || !zoomEl) return;
    const prev = readerZoom;
    const zoom = Math.min(READER_ZOOM_MAX, Math.max(READER_ZOOM_MIN, next));
    if (Math.abs(zoom - prev) < 0.001) return;

    const rect = scroller.getBoundingClientRect();
    const ax = anchor ? anchor.x - rect.left : scroller.clientWidth / 2;
    const ay = anchor ? anchor.y - rect.top : scroller.clientHeight / 2;
    const contentX = scroller.scrollLeft + ax;
    const contentY = scroller.scrollTop + ay;
    const ratio = zoom / prev;

    readerZoom = zoom;
    zoomEl.style.zoom = String(zoom);
    if (els.writingZoomHint) {
      els.writingZoomHint.textContent = `${Math.round(zoom * 100)}%`;
    }

    scroller.scrollLeft = contentX * ratio - ax;
    scroller.scrollTop = contentY * ratio - ay;
  }

  function resetReaderZoom() {
    readerZoom = 1;
    if (els.writingReaderZoom) els.writingReaderZoom.style.zoom = "1";
    if (els.writingZoomHint) els.writingZoomHint.textContent = "100%";
    if (els.writingReaderBody) {
      els.writingReaderBody.scrollLeft = 0;
      els.writingReaderBody.scrollTop = 0;
    }
  }

  function openWritingReader() {
    if (!selectedWritingId || writingModelEditing) return;
    const src = els.writingModel?.value || "";
    const target = els.writingReaderZoom || els.writingReaderBody;
    if (target) {
      target.innerHTML = src.trim()
        ? renderModelMarkdown(src)
        : `<p class="empty">暂无范文内容。</p>`;
    }
    resetReaderZoom();
    if (els.writingReader) els.writingReader.hidden = false;
  }

  function closeWritingReader() {
    if (els.writingReader) els.writingReader.hidden = true;
    resetReaderZoom();
  }

  function renderWritingList() {
    if (!els.writingList) return;
    if (els.writingCount) els.writingCount.textContent = String(writings.length);
    if (!writings.length) {
      els.writingList.innerHTML = `<p class="empty">还没有条目。点击「新建作文」开始。</p>`;
      return;
    }
    els.writingList.innerHTML = writings
      .map((w) => {
        const active = selectedWritingId === w.id ? " active" : "";
        const hasModel = !!(w.model_text || "").trim();
        const imgNote =
          (w.prompt_image_count || 0) > 0 ? ` · ${w.prompt_image_count} 张截图` : "";
        return `<button type="button" class="writing-row${active}" data-id="${w.id}">
          <span class="term">${escapeHtml((w.prompt || "").trim() || "未填写题目")}</span>
          <span class="meta">${escapeHtml(formatWritingTime(w.updated_at))}${imgNote}${
            hasModel ? " · 已有范文" : " · 待导入范文"
          }</span>
        </button>`;
      })
      .join("");
  }

  function fillWritingEditor(w) {
    if (!w || !els.writingEditor) return;
    els.writingEditor.hidden = false;
    if (els.writingEmpty) els.writingEmpty.hidden = true;
    if (els.btnDeleteWriting) els.btnDeleteWriting.disabled = false;
    els.writingPrompt.value = w.prompt || "";
    els.writingModel.value = w.model_text || "";
    promptImages = Array.isArray(w.prompt_images)
      ? w.prompt_images.map((img) => ({
          mime: img.mime || "image/png",
          data: img.data || "",
        }))
      : [];
    renderPromptImages();
    if (els.writingModelFilename) {
      const name = (w.model_filename || "").trim();
      els.writingModelFilename.hidden = !name;
      els.writingModelFilename.textContent = name ? `来自 ${name}` : "";
    }
    setModelEditMode(false);
    writingDirty = false;
    setWritingSaveHint("");
  }

  function clearWritingEditor() {
    selectedWritingId = null;
    closeWritingReader();
    closeImageLightbox();
    if (els.writingEditor) els.writingEditor.hidden = true;
    if (els.writingEmpty) els.writingEmpty.hidden = false;
    if (els.btnDeleteWriting) els.btnDeleteWriting.disabled = true;
    if (els.writingPrompt) els.writingPrompt.value = "";
    if (els.writingModel) els.writingModel.value = "";
    promptImages = [];
    renderPromptImages();
    if (els.writingModelFilename) {
      els.writingModelFilename.hidden = true;
      els.writingModelFilename.textContent = "";
    }
    setModelEditMode(false);
    writingDirty = false;
    setWritingSaveHint("");
    renderWritingList();
  }

  async function loadWritings({ silent } = {}) {
    const list = await api("/api/writings");
    writings = Array.isArray(list) ? list : [];
    if (
      selectedWritingId &&
      !writings.some((w) => w.id === selectedWritingId)
    ) {
      clearWritingEditor();
    } else {
      renderWritingList();
      if (selectedWritingId && !silent && !writingDirty) {
        const detail = await api(`/api/writings/${selectedWritingId}`);
        fillWritingEditor(detail);
      }
    }
  }

  async function selectWriting(id) {
    if (selectedWritingId === id) return;
    if (writingDirty) {
      try {
        await saveWritingNow();
      } catch (err) {
        toast(err.message);
        return;
      }
    }
    selectedWritingId = id;
    try {
      const detail = await api(`/api/writings/${id}`);
      const idx = writings.findIndex((x) => x.id === id);
      if (idx >= 0) {
        writings[idx] = {
          ...writings[idx],
          prompt: detail.prompt,
          model_text: detail.model_text,
          model_filename: detail.model_filename,
          prompt_image_count: (detail.prompt_images || []).length,
          updated_at: detail.updated_at,
        };
      }
      fillWritingEditor(detail);
      renderWritingList();
    } catch (err) {
      toast(err.message);
      clearWritingEditor();
    }
  }

  function scheduleWritingSave() {
    if (!selectedWritingId) return;
    writingDirty = true;
    setWritingSaveHint("未保存…");
    clearTimeout(writingSaveTimer);
    writingSaveTimer = setTimeout(() => {
      saveWritingNow().catch((err) => toast(err.message));
    }, 800);
  }

  async function saveWritingNow() {
    if (!selectedWritingId || writingSaving) return;
    clearTimeout(writingSaveTimer);
    writingSaving = true;
    setWritingSaveHint("保存中…");
    try {
      const updated = await api(`/api/writings/${selectedWritingId}`, {
        method: "PUT",
        body: JSON.stringify({
          prompt: els.writingPrompt?.value || "",
          prompt_images: promptImages,
          body: "",
          model_text: els.writingModel?.value || "",
        }),
      });
      writingDirty = false;
      promptImages = Array.isArray(updated.prompt_images) ? updated.prompt_images : promptImages;
      const idx = writings.findIndex((w) => w.id === selectedWritingId);
      const summary = {
        ...updated,
        prompt_image_count: (updated.prompt_images || promptImages).length,
        prompt_images: undefined,
      };
      if (idx >= 0) writings[idx] = summary;
      else writings.unshift(summary);
      writings.sort((a, b) => String(b.updated_at).localeCompare(String(a.updated_at)));
      renderWritingList();
      renderPromptImages();
      if (els.writingModelFilename) {
        const name = (updated.model_filename || "").trim();
        els.writingModelFilename.hidden = !name;
        els.writingModelFilename.textContent = name ? `来自 ${name}` : "";
      }
      if (!writingModelEditing) refreshModelPreview();
      setWritingSaveHint("已保存");
    } catch (err) {
      setWritingSaveHint("保存失败");
      throw err;
    } finally {
      writingSaving = false;
    }
  }

  async function createWriting() {
    if (writingDirty) {
      try {
        await saveWritingNow();
      } catch (err) {
        toast(err.message);
        return;
      }
    }
    const created = await api("/api/writings", {
      method: "POST",
      body: JSON.stringify({ prompt: "", prompt_images: [], body: "", model_text: "" }),
    });
    writings.unshift({
      ...created,
      prompt_image_count: 0,
      prompt_images: undefined,
    });
    selectedWritingId = created.id;
    fillWritingEditor(created);
    renderWritingList();
    els.writingPrompt?.focus();
    toast("已新建作文");
  }

  async function deleteWriting() {
    if (!selectedWritingId) return;
    if (!confirm("确定删除这篇作文？")) return;
    clearTimeout(writingSaveTimer);
    writingDirty = false;
    const id = selectedWritingId;
    await api(`/api/writings/${id}`, { method: "DELETE" });
    writings = writings.filter((w) => w.id !== id);
    clearWritingEditor();
    toast("已删除");
  }

  async function uploadWritingModel(file) {
    if (!selectedWritingId || !file) return;
    if (writingDirty) {
      await saveWritingNow();
    }
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(`/api/writings/${selectedWritingId}/model-file`, {
      method: "POST",
      body: form,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(data.error || `上传失败 (${res.status})`);
    }
    const idx = writings.findIndex((w) => w.id === selectedWritingId);
    if (idx >= 0) writings[idx] = data;
    else writings.unshift(data);
    fillWritingEditor(data);
    renderWritingList();
    toast("范文已导入");
  }

  async function openWritingPage() {
    showPage("writing");
    try {
      await loadWritings();
      if (!selectedWritingId && writings.length) {
        await selectWriting(writings[0].id);
      } else if (!writings.length) {
        clearWritingEditor();
      }
    } catch (err) {
      toast(err.message);
    }
  }

  let voicesCache = null;
  let currentAudio = null;

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

  function normalizeAudioURL(u) {
    const s = String(u || "").trim();
    if (!s) return "";
    if (s.startsWith("//")) return `https:${s}`;
    return s;
  }

  function stopSpeech() {
    if (window.speechSynthesis) window.speechSynthesis.cancel();
    if (currentAudio) {
      currentAudio.pause();
      currentAudio.src = "";
      currentAudio = null;
    }
  }

  function playAudioURL(url) {
    return new Promise((resolve, reject) => {
      stopSpeech();
      const audio = new Audio(url);
      currentAudio = audio;
      const cleanup = () => {
        if (currentAudio === audio) currentAudio = null;
      };
      audio.onended = () => {
        cleanup();
        resolve();
      };
      audio.onerror = () => {
        cleanup();
        reject(new Error("audio error"));
      };
      audio.play().catch((err) => {
        cleanup();
        reject(err);
      });
    });
  }

  function speakWithSystem(content) {
    if (!window.speechSynthesis) {
      toast("当前环境不支持语音朗读");
      return;
    }
    stopSpeech();
    const utter = new SpeechSynthesisUtterance(content);
    utter.lang = "en-US";
    const voice = pickEnglishVoice();
    if (voice) utter.voice = voice;
    utter.pitch = 1;
    utter.onerror = () => toast("朗读失败，请检查系统语音设置");
    window.speechSynthesis.speak(utter);
  }

  async function speakEnglish(text, audioUrl) {
    const content = String(text || "").trim();
    if (!content) return;

    const direct = normalizeAudioURL(audioUrl);
    if (direct) {
      try {
        await playAudioURL(direct);
        return;
      } catch (_) {
        /* fall through */
      }
      try {
        const qs = new URLSearchParams({ q: content, src: direct });
        await playAudioURL(`/api/tts?${qs.toString()}`);
        return;
      } catch (_) {
        /* fall through */
      }
    }

    try {
      await playAudioURL(`/api/tts?q=${encodeURIComponent(content)}`);
      return;
    } catch (_) {
      /* fall through */
    }

    speakWithSystem(content);
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

  function extraFP(w) {
    const forms = (w.related_forms || []).map((f) => `${f.pos || ""}:${f.word || ""}:${f.meaning_zh || ""}`).join(",");
    const phrases = (w.phrases || []).map((p) => `${p.phrase || ""}:${p.meaning_zh || ""}`).join(",");
    return `${forms}|${phrases}`;
  }

  function renderExtraList(title, items) {
    if (!items) return "";
    return `<section class="extra-block">
      <h4>${title}</h4>
      <ul class="extra-list">${items}</ul>
    </section>`;
  }

  function renderForms(forms) {
    const list = (forms || []).filter((f) => (f.word || "").trim());
    if (!list.length) return "";
    const items = list
      .map((f) => {
        const pos = (f.pos || "").trim();
        const zh = (f.meaning_zh || "").trim();
        return `<li>
          ${pos ? `<span class="pos-tag">${escapeHtml(pos)}</span>` : ""}
          <span class="extra-term">${escapeHtml(f.word)}</span>
          ${zh ? `<span class="extra-zh">${escapeHtml(zh)}</span>` : ""}
        </li>`;
      })
      .join("");
    return renderExtraList("词形变化", items);
  }

  function renderPhrases(phrases) {
    const list = (phrases || []).filter((p) => (p.phrase || "").trim());
    if (!list.length) return "";
    const items = list
      .map((p) => {
        const zh = (p.meaning_zh || "").trim();
        return `<li>
          <span class="extra-term">${escapeHtml(p.phrase)}</span>
          ${zh ? `<span class="extra-zh">${escapeHtml(zh)}</span>` : ""}
        </li>`;
      })
      .join("");
    return renderExtraList("常用短语", items);
  }

  function renderMeaningBody(w) {
    const example = w.example_en
      ? `<p class="example">${escapeHtml(w.example_en)}${
          w.example_zh ? `<br />${escapeHtml(w.example_zh)}` : ""
        }<button type="button" class="btn ghost sm speak-inline" data-speak="${escapeHtml(
          w.example_en
        )}" title="朗读例句">朗读例句</button></p>`
      : "";
    const meaningZh = (w.meaning_zh || "").trim();
    return `
      ${meaningZh ? `<p class="meaning-zh">${escapeHtml(meaningZh)}</p>` : ""}
      <p class="meaning-en">${escapeHtml(w.meaning_en || "")}</p>
      ${example}
      ${renderForms(w.related_forms)}
      ${renderPhrases(w.phrases)}`;
  }

  function renderDetailHTML(w) {
    const meta = renderMeta(w);
    const hasMeaning = (w.meaning_zh || "").trim() || (w.meaning_en || "").trim();
    let body = "";
    if (w.status === "pending" && !hasMeaning) {
      body = `<p class="meaning-zh pending">释义生成中…</p>`;
    } else if (w.status === "error" && !hasMeaning) {
      body = `<p class="meaning-zh error">释义失败：${escapeHtml(w.error_msg || "未知错误")}</p>
        <div class="word-actions"><button type="button" class="btn ghost sm" data-retry="${w.id}">重试</button></div>`;
    } else {
      const note =
        w.status === "pending"
          ? `<p class="meaning-zh pending">正在补全词形与短语…</p>`
          : w.status === "error"
            ? `<p class="meaning-zh error">释义失败：${escapeHtml(w.error_msg || "未知错误")}</p>
        <div class="word-actions"><button type="button" class="btn ghost sm" data-retry="${w.id}">重试</button></div>`
            : "";
      body = `${note}${renderMeaningBody(w)}`;
    }
    return `
      <div class="detail-head">
        <div>
          <div class="term-row">
            <h3 class="term">${escapeHtml(w.term)}</h3>
            <button type="button" class="btn-speak" data-speak="${escapeHtml(w.term)}" data-audio="${escapeHtml(
              w.audio_url || ""
            )}" title="朗读单词" aria-label="朗读单词">
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
        (w.meaning_en || "").toLowerCase().includes(q) ||
        extraFP(w).toLowerCase().includes(q)
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
      model: "deepseek-flash",
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
      await refreshAll();
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
    const onWriting =
      isAuxWritingWindow || (els.pageWriting && !els.pageWriting.hidden);
    if (onWriting) {
      await Promise.all([
        loadStatus({ silent }),
        loadWritings({ silent: true }).catch(() => {}),
      ]);
      return;
    }
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

  let lastTranslation = "";

  function setTranslateOpen(open) {
    if (!els.translatePanel || !els.btnToggleTranslate) return;
    els.translatePanel.hidden = !open;
    els.btnToggleTranslate.setAttribute("aria-expanded", open ? "true" : "false");
    els.btnToggleTranslate.classList.toggle("active", open);
    if (open) {
      els.translateInput?.focus();
    }
  }

  function directionLabel(dir) {
    if (dir === "zh2en") return "中 → 英";
    if (dir === "en2zh") return "英 → 中";
    return "自动";
  }

  async function runTranslate() {
    const text = (els.translateInput?.value || "").trim();
    if (!text) {
      toast("请输入要翻译的文本");
      els.translateInput?.focus();
      return;
    }
    const direction = els.translateDirection?.value || "auto";
    els.btnDoTranslate.disabled = true;
    els.translateOutput.classList.add("loading");
    els.translateOutput.innerHTML = `<p class="empty">翻译中…</p>`;
    els.translateMeta.hidden = true;
    els.btnCopyTranslation.hidden = true;
    lastTranslation = "";
    try {
      const data = await api("/api/translate", {
        method: "POST",
        body: JSON.stringify({ text, direction }),
      });
      lastTranslation = (data.translation || "").trim();
      if (!lastTranslation) {
        els.translateOutput.innerHTML = `<p class="empty">未得到译文</p>`;
        return;
      }
      els.translateOutput.textContent = lastTranslation;
      els.btnCopyTranslation.hidden = false;
      if (els.translateMeta) {
        els.translateMeta.hidden = false;
        els.translateMeta.textContent = `方向：${directionLabel(data.direction)}`;
      }
    } catch (err) {
      els.translateOutput.innerHTML = `<p class="empty">${escapeHtml(err.message)}</p>`;
      toast(err.message);
    } finally {
      els.translateOutput.classList.remove("loading");
      els.btnDoTranslate.disabled = false;
    }
  }

  function swapTranslate() {
    const src = (els.translateInput?.value || "").trim();
    const dst = lastTranslation || (els.translateOutput?.textContent || "").trim();
    if (!dst || dst === "译文将显示在这里" || dst === "翻译中…" || dst === "未得到译文") {
      toast("没有可交换的译文");
      return;
    }
    els.translateInput.value = dst;
    lastTranslation = src;
    els.translateOutput.textContent = src || "";
    els.btnCopyTranslation.hidden = !src;
    const dir = els.translateDirection?.value;
    if (dir === "en2zh") els.translateDirection.value = "zh2en";
    else if (dir === "zh2en") els.translateDirection.value = "en2zh";
    if (els.translateMeta) {
      els.translateMeta.hidden = false;
      els.translateMeta.textContent = "已交换原文与译文";
    }
  }

  async function copyTranslation() {
    const text = lastTranslation || (els.translateOutput?.textContent || "").trim();
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      toast("译文已复制");
    } catch {
      toast("复制失败，请手动选择文本");
    }
  }

  function quizTypeLabel(type) {
    switch (type) {
      case "word_to_def":
        return "看词选义";
      case "def_to_word":
        return "看义选词";
      case "closest_meaning":
        return "近义匹配";
      case "zh_to_word":
        return "中文选词";
      case "word_to_zh":
        return "看词选中文";
      default:
        return "";
    }
  }

  async function reportQuizAnswer(wordId, correct) {
    if (!wordId) return;
    try {
      await api("/api/quiz/result", {
        method: "POST",
        body: JSON.stringify({ results: [{ id: wordId, correct: !!correct }] }),
      });
    } catch (_) {
      /* non-blocking */
    }
  }

  async function createQuiz() {
    els.btnNewQuiz.disabled = true;
    els.quizArea.innerHTML = `<p class="empty">正在出题…</p>`;
    quizScore = { correct: 0, total: 0 };
    try {
      const data = await api("/api/quiz", {
        method: "POST",
        body: JSON.stringify({ count: 5 }),
      });
      const qs = data.questions || [];
      if (!qs.length) {
        els.quizArea.innerHTML = `<p class="empty">没有生成题目，请稍后再试。</p>`;
        return;
      }
      quizScore.total = qs.length;
      els.quizArea.innerHTML = `
        <div class="score-banner" id="scoreBanner">进度：0 / ${qs.length}</div>
        ${qs
          .map((q, qi) => {
            const typeLabel = quizTypeLabel(q.type);
            const stem = escapeHtml(q.question || "").replace(/\n/g, "<br />");
            return `
          <div class="quiz-q" data-qi="${qi}" data-word-id="${q.id || ""}">
            <h3>${qi + 1}. ${
              typeLabel ? `<span class="quiz-type">${escapeHtml(typeLabel)}</span>` : ""
            }${stem}</h3>
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
          </div>`;
          })
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
        ? `完成！得分 ${quizScore.correct} / ${quizScore.total}`
        : `进度：${done} / ${quizScore.total} · 正确 ${quizScore.correct}`;
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
      speakEnglish(speak.dataset.speak, speak.dataset.audio || "");
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

  els.btnGoWriting?.addEventListener("click", () => {
    if (openWritingWindowPreferred()) return;
    openWritingPage().catch((err) => toast(err.message));
  });

  els.btnBackNotebook.addEventListener("click", () => {
    showPage("notebook");
  });

  els.btnBackFromWriting?.addEventListener("click", async () => {
    if (writingDirty) {
      try {
        await saveWritingNow();
      } catch (err) {
        toast(err.message);
        return;
      }
    }
    if (isAuxWritingWindow) {
      if (!showNotebookWindowPreferred()) {
        location.href = "/";
      }
      return;
    }
    showPage("notebook");
  });

  els.btnNewWriting?.addEventListener("click", () => {
    createWriting().catch((err) => toast(err.message));
  });

  els.btnDeleteWriting?.addEventListener("click", () => {
    deleteWriting().catch((err) => toast(err.message));
  });

  els.writingList?.addEventListener("click", (e) => {
    const row = e.target.closest(".writing-row");
    if (!row) return;
    selectWriting(Number(row.dataset.id)).catch((err) => toast(err.message));
  });

  ["input", "change"].forEach((evt) => {
    els.writingPrompt?.addEventListener(evt, scheduleWritingSave);
    els.writingModel?.addEventListener(evt, scheduleWritingSave);
  });

  els.writingPromptField?.addEventListener("paste", (e) => {
    const items = Array.from(e.clipboardData?.items || []);
    const imageItems = items.filter((it) => it.type && it.type.startsWith("image/"));
    if (!imageItems.length) return;
    e.preventDefault();
    if (!selectedWritingId) {
      toast("请先新建或选择一篇作文");
      return;
    }
    const files = imageItems.map((it) => it.getAsFile()).filter(Boolean);
    addPromptImageFiles(files).catch((err) => toast(err.message));
  });

  els.btnAddPromptImage?.addEventListener("click", () => {
    if (!selectedWritingId) {
      toast("请先新建或选择一篇作文");
      return;
    }
    els.writingPromptImageFile?.click();
  });

  els.writingPromptImageFile?.addEventListener("change", () => {
    const files = els.writingPromptImageFile.files;
    els.writingPromptImageFile.value = "";
    if (!files?.length) return;
    addPromptImageFiles(files).catch((err) => toast(err.message));
  });

  els.writingPromptImages?.addEventListener("click", (e) => {
    const btn = e.target.closest("[data-remove-image]");
    if (btn) {
      const idx = Number(btn.dataset.removeImage);
      if (!Number.isFinite(idx)) return;
      promptImages.splice(idx, 1);
      renderPromptImages();
      scheduleWritingSave();
      return;
    }
    const img = e.target.closest("img");
    if (img?.src) openImageLightbox(img.src);
  });

  els.btnCloseImageLightbox?.addEventListener("click", closeImageLightbox);

  els.writingImageLightbox?.addEventListener("click", (e) => {
    if (e.target === els.writingImageLightbox || e.target?.id === "writingLightboxStage") {
      if (els.writingImageLightbox?.dataset.dragged === "1") {
        els.writingImageLightbox.dataset.dragged = "";
        return;
      }
      closeImageLightbox();
    }
  });

  (() => {
    const box = els.writingImageLightbox;
    if (!box) return;
    let dragging = false;
    let moved = false;
    let startX = 0;
    let startY = 0;
    let startLeft = 0;
    let startTop = 0;
    let pointerId = null;

    box.addEventListener("pointerdown", (e) => {
      if (e.button !== 0) return;
      if (e.target.closest("button")) return;
      dragging = true;
      moved = false;
      startX = e.clientX;
      startY = e.clientY;
      startLeft = box.scrollLeft;
      startTop = box.scrollTop;
      pointerId = e.pointerId;
      box.dataset.dragged = "";
      box.setPointerCapture(e.pointerId);
      box.classList.add("is-dragging");
    });

    box.addEventListener("pointermove", (e) => {
      if (!dragging) return;
      const dx = e.clientX - startX;
      const dy = e.clientY - startY;
      if (Math.abs(dx) > 2 || Math.abs(dy) > 2) {
        moved = true;
        box.dataset.dragged = "1";
      }
      box.scrollLeft = startLeft - dx;
      box.scrollTop = startTop - dy;
    });

    const endDrag = () => {
      if (!dragging) return;
      dragging = false;
      box.classList.remove("is-dragging");
      if (pointerId != null) {
        try {
          box.releasePointerCapture(pointerId);
        } catch {
          /* ignore */
        }
        pointerId = null;
      }
    };

    box.addEventListener("pointerup", endDrag);
    box.addEventListener("pointercancel", endDrag);
  })();

  els.writingModel?.addEventListener("input", () => {
    if (!writingModelEditing) refreshModelPreview();
  });

  els.btnToggleModelEdit?.addEventListener("click", () => {
    if (!selectedWritingId) {
      toast("请先新建或选择一篇作文");
      return;
    }
    if (writingModelEditing) {
      setModelEditMode(false);
      scheduleWritingSave();
      return;
    }
    setModelEditMode(true);
    els.writingModel?.focus();
  });

  els.btnExpandModel?.addEventListener("click", () => {
    if (!selectedWritingId) {
      toast("请先新建或选择一篇作文");
      return;
    }
    openWritingReader();
  });

  els.writingModelPreview?.addEventListener("click", () => {
    openWritingReader();
  });

  els.btnCloseWritingReader?.addEventListener("click", closeWritingReader);

  els.writingReader?.addEventListener("click", (e) => {
    if (e.target === els.writingReader) closeWritingReader();
  });

  (() => {
    const scroller = els.writingReaderBody;
    if (!scroller) return;
    let dragging = false;
    let moved = false;
    let startX = 0;
    let startY = 0;
    let startLeft = 0;
    let startTop = 0;
    let pointerId = null;

    scroller.addEventListener(
      "wheel",
      (e) => {
        // Trackpad pinch / Ctrl+滚轮：只放大内部文字，外层容器不变
        if (e.ctrlKey || e.metaKey) {
          e.preventDefault();
          const factor = Math.exp(-e.deltaY * 0.01);
          setReaderZoom(readerZoom * factor, { x: e.clientX, y: e.clientY });
          return;
        }
        // 普通双指滑动：在容器内平移
        if (e.shiftKey) {
          scroller.scrollLeft += e.deltaY;
        } else {
          scroller.scrollTop += e.deltaY;
          scroller.scrollLeft += e.deltaX;
        }
      },
      { passive: false }
    );

    scroller.addEventListener("pointerdown", (e) => {
      if (e.button !== 0) return;
      if (e.target.closest("a, button, input, textarea, select")) return;
      dragging = true;
      moved = false;
      startX = e.clientX;
      startY = e.clientY;
      startLeft = scroller.scrollLeft;
      startTop = scroller.scrollTop;
      pointerId = e.pointerId;
      scroller.setPointerCapture(e.pointerId);
      scroller.classList.add("is-dragging");
    });

    scroller.addEventListener("pointermove", (e) => {
      if (!dragging) return;
      e.preventDefault();
      const dx = e.clientX - startX;
      const dy = e.clientY - startY;
      if (Math.abs(dx) > 2 || Math.abs(dy) > 2) moved = true;
      scroller.scrollLeft = startLeft - dx;
      scroller.scrollTop = startTop - dy;
    });

    const endDrag = (e) => {
      if (!dragging) return;
      dragging = false;
      scroller.classList.remove("is-dragging");
      if (pointerId != null) {
        try {
          scroller.releasePointerCapture(pointerId);
        } catch {
          /* ignore */
        }
        pointerId = null;
      }
      if (moved) e.preventDefault();
    };

    scroller.addEventListener("pointerup", endDrag);
    scroller.addEventListener("pointercancel", endDrag);
    scroller.addEventListener(
      "click",
      (e) => {
        if (!moved) return;
        e.preventDefault();
        e.stopPropagation();
        moved = false;
      },
      true
    );
  })();

  els.btnUploadModel?.addEventListener("click", () => {
    if (!selectedWritingId) {
      toast("请先新建或选择一篇作文");
      return;
    }
    els.writingModelFile?.click();
  });

  els.writingModelFile?.addEventListener("change", () => {
    const file = els.writingModelFile.files?.[0];
    els.writingModelFile.value = "";
    if (!file) return;
    uploadWritingModel(file).catch((err) => toast(err.message));
  });

  els.btnNewQuiz.addEventListener("click", createQuiz);

  els.btnToggleTranslate?.addEventListener("click", () => {
    const open = els.translatePanel?.hidden !== false;
    setTranslateOpen(open);
  });

  els.btnDoTranslate?.addEventListener("click", () => {
    runTranslate().catch((err) => toast(err.message));
  });

  els.btnSwapTranslate?.addEventListener("click", swapTranslate);
  els.btnCopyTranslation?.addEventListener("click", () => {
    copyTranslation().catch((err) => toast(err.message));
  });

  els.translateInput?.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      runTranslate().catch((err) => toast(err.message));
    }
  });

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
    if (e.key === "Escape" && els.writingImageLightbox && !els.writingImageLightbox.hidden) {
      closeImageLightbox();
      return;
    }
    if (e.key === "Escape" && els.writingReader && !els.writingReader.hidden) {
      closeWritingReader();
      return;
    }
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
    const isCorrect = chosen === correct;
    const options = block.querySelectorAll(".opt");
    options.forEach((o) => {
      o.disabled = true;
      if (Number(o.dataset.oi) === correct) o.classList.add("correct");
    });
    if (isCorrect) {
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
    const wordId = Number(block.dataset.wordId || 0);
    reportQuizAnswer(wordId, isCorrect);
  });

  if (isAuxWritingWindow) {
    document.body.classList.add("aux-writing");
    document.title = "PhraseMate 写作本";
    if (els.btnBackFromWriting) {
      els.btnBackFromWriting.textContent = "显示生词本";
    }
    if (els.btnGoWriting) els.btnGoWriting.hidden = true;
    if (els.btnGoQuiz) els.btnGoQuiz.hidden = true;
    openWritingPage().catch((err) => toast(err.message));
    loadStatus();
  } else {
    showPage("notebook");
    loadStatus();
    loadWords().catch((err) => toast(err.message));
  }
})();
