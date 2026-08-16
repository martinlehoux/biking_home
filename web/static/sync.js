export const mountSync = () => {
  const form = /** @type {HTMLFormElement | null} */ (document.getElementById("sync-form"));
  if (!form) return;
  /** @type {HTMLButtonElement} */
  const button = form.querySelector("button[type=submit]");
  /** @type {HTMLElement} */
  const progressPanel = document.getElementById("sync-progress");
  const progressBar = /** @type {HTMLProgressElement} */ (document.getElementById("sync-progress-bar"));
  /** @type {HTMLElement} */
  const progressCount = document.getElementById("sync-progress-count");
  /** @type {HTMLElement} */
  const progressDetails = document.getElementById("sync-progress-details");
  /** @type {HTMLElement} */
  const notice = document.getElementById("sync-notice");
  /** @type {HTMLElement} */
  const error = document.getElementById("sync-error");

  const showError = (message) => {
    error.textContent = message;
    error.hidden = false;
  };
  const updateProgress = (progress) => {
    progressPanel.hidden = false;
    progressBar.max = Math.max(progress.total, 1);
    progressBar.value = progress.completed;
    progressCount.textContent = `${progress.completed} / ${progress.total}`;
    progressDetails.textContent = `${progress.imported} imported · ${progress.skipped} skipped`;
  };
  const handleEvent = (eventName, data) => {
    if (eventName === "progress") {
      updateProgress(data);
      return false;
    }
    if (eventName === "complete") {
      updateProgress(data);
      notice.textContent = `Sync complete: ${data.imported} imported, ${data.skipped} skipped.`;
      notice.hidden = false;
      return true;
    }
    if (eventName === "error") {
      if (data.progress) updateProgress(data.progress);
      throw new Error(data.message || "Strava sync failed");
    }
    return false;
  };
  const consumeEvents = async (response) => {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    let completed = false;
    const consumeBlock = (block) => {
      let eventName = "message";
      let data = "";
      for (const line of block.split(/\r?\n/)) {
        if (line.startsWith("event: ")) eventName = line.slice(7);
        if (line.startsWith("data: ")) data += line.slice(6);
      }
      if (data) completed = handleEvent(eventName, JSON.parse(data)) || completed;
    };
    while (true) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
      const blocks = buffer.split(/\r?\n\r?\n/);
      buffer = blocks.pop();
      for (const block of blocks) consumeBlock(block);
      if (done) break;
    }
    if (buffer.trim()) consumeBlock(buffer);
    if (!completed) throw new Error("Strava sync ended before completion");
  };

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    button.disabled = true;
    button.textContent = "Syncing...";
    notice.hidden = true;
    error.hidden = true;
    progressPanel.hidden = false;
    progressCount.textContent = "0 / 0";
    progressDetails.textContent = "Preparing import...";
    try {
      const response = await fetch(form.action, {
        method: "POST",
        body: new FormData(form),
        headers: { Accept: "text/event-stream" },
      });
      if (response.redirected) {
        window.location.assign(response.url);
        return;
      }
      if (!response.ok) throw new Error((await response.text()) || `Sync failed (HTTP ${response.status})`);
      if (!response.body) throw new Error("The browser does not support streaming responses");
      await consumeEvents(response);
    } catch (caught) {
      showError(caught instanceof Error ? caught.message : "Strava sync failed");
    } finally {
      button.disabled = false;
      button.textContent = "Sync rides";
    }
  });
};

mountSync();
