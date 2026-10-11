(() => {
  function escapeHtml(s) {
    return String(s ?? "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function inlineMD(text) {
    let s = escapeHtml(text);
    s = s.replace(/`([^`]+)`/g, "<code>$1</code>");
    s = s.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
    s = s.replace(/__([^_]+)__/g, "<strong>$1</strong>");
    s = s.replace(/(^|[\s(])\*([^*\n]+)\*(?=[\s).,!?;:]|$)/g, "$1<em>$2</em>");
    s = s.replace(/(^|[\s(])_([^_\n]+)_(?=[\s).,!?;:]|$)/g, "$1<em>$2</em>");
    s = s.replace(
      /\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g,
      '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>'
    );
    return s;
  }

  function renderMarkdown(src) {
    const text = String(src || "").replace(/\r\n/g, "\n");
    if (!text.trim()) return "";

    const lines = text.split("\n");
    const out = [];
    let i = 0;
    let para = [];
    let listType = null;
    let listItems = [];

    const flushPara = () => {
      if (!para.length) return;
      out.push(`<p>${inlineMD(para.join(" "))}</p>`);
      para = [];
    };

    const flushList = () => {
      if (!listType) return;
      const tag = listType;
      out.push(`<${tag}>${listItems.map((item) => `<li>${inlineMD(item)}</li>`).join("")}</${tag}>`);
      listType = null;
      listItems = [];
    };

    while (i < lines.length) {
      const line = lines[i];
      const trimmed = line.trim();

      if (trimmed.startsWith("```")) {
        flushPara();
        flushList();
        i += 1;
        const code = [];
        while (i < lines.length && !lines[i].trim().startsWith("```")) {
          code.push(lines[i]);
          i += 1;
        }
        if (i < lines.length) i += 1;
        out.push(`<pre><code>${escapeHtml(code.join("\n"))}</code></pre>`);
        continue;
      }

      if (!trimmed) {
        flushPara();
        flushList();
        i += 1;
        continue;
      }

      if (/^(-{3,}|\*{3,}|_{3,})$/.test(trimmed)) {
        flushPara();
        flushList();
        out.push("<hr />");
        i += 1;
        continue;
      }

      const heading = /^(#{1,6})\s+(.+)$/.exec(trimmed);
      if (heading) {
        flushPara();
        flushList();
        const level = heading[1].length;
        out.push(`<h${level}>${inlineMD(heading[2])}</h${level}>`);
        i += 1;
        continue;
      }

      if (/^>\s?/.test(trimmed)) {
        flushPara();
        flushList();
        const quote = [];
        while (i < lines.length && /^>\s?/.test(lines[i].trim())) {
          quote.push(lines[i].trim().replace(/^>\s?/, ""));
          i += 1;
        }
        out.push(`<blockquote><p>${inlineMD(quote.join(" "))}</p></blockquote>`);
        continue;
      }

      const ul = /^[-*+]\s+(.+)$/.exec(trimmed);
      if (ul) {
        flushPara();
        if (listType && listType !== "ul") flushList();
        listType = "ul";
        listItems.push(ul[1]);
        i += 1;
        continue;
      }

      const ol = /^(\d+)\.\s+(.+)$/.exec(trimmed);
      if (ol) {
        flushPara();
        if (listType && listType !== "ol") flushList();
        listType = "ol";
        listItems.push(ol[2]);
        i += 1;
        continue;
      }

      flushList();
      para.push(trimmed);
      i += 1;
    }

    flushPara();
    flushList();
    return out.join("\n");
  }

  window.phrasemateMarkdown = { render: renderMarkdown, escapeHtml };
})();
