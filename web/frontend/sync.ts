import type { SyncErrorEvent, SyncProgress } from "./types.js";
import { requireElement } from "./dom.js";

const isSyncProgress = (data: SyncProgress | SyncErrorEvent): data is SyncProgress => "total" in data;

export const mountSync = (): void => {
  const form = document.getElementById("sync-form") as HTMLFormElement | null;
  if (!form) return;
  const button = requireElement(form.querySelector<HTMLButtonElement>("button[type=submit]"), "sync submit button");
  const progressPanel = requireElement(document.getElementById("sync-progress"), "sync progress");
  const progressBar = requireElement(document.getElementById("sync-progress-bar") as HTMLProgressElement | null, "sync progress bar");
  const progressCount = requireElement(document.getElementById("sync-progress-count"), "sync progress count");
  const progressDetails = requireElement(document.getElementById("sync-progress-details"), "sync progress details");
  const notice = requireElement(document.getElementById("sync-notice"), "sync notice");
  const error = requireElement(document.getElementById("sync-error"), "sync error");

  const showError = (message: string): void => {
    error.textContent = message;
    error.hidden = false;
  };
  const updateProgress = (progress: SyncProgress): void => {
    progressPanel.hidden = false;
    progressBar.max = Math.max(progress.total, 1);
    progressBar.value = progress.completed;
    progressCount.textContent = `${progress.completed} / ${progress.total}`;
    progressDetails.textContent = `${progress.imported} imported · ${progress.skipped} skipped`;
  };
  const handleEvent = (eventName: string, data: SyncProgress | SyncErrorEvent): boolean => {
    if (eventName === "progress") {
      if (!isSyncProgress(data)) throw new Error("Invalid Strava sync progress event");
      updateProgress(data);
      return false;
    }
    if (eventName === "complete") {
      if (!isSyncProgress(data)) throw new Error("Invalid Strava sync completion event");
      updateProgress(data);
      notice.textContent = `Sync complete: ${data.imported} imported, ${data.skipped} skipped.`;
      notice.hidden = false;
      return true;
    }
    if (eventName === "error") {
      if ("progress" in data && data.progress) updateProgress(data.progress);
      throw new Error("message" in data ? data.message || "Strava sync failed" : "Strava sync failed");
    }
    return false;
  };
  const consumeEvents = async (response: Response): Promise<void> => {
    if (!response.body) throw new Error("The browser does not support streaming responses");
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    let completed = false;
    const consumeBlock = (block: string): void => {
      let eventName = "message";
      let data = "";
      for (const line of block.split(/\r?\n/)) {
        if (line.startsWith("event: ")) eventName = line.slice(7);
        if (line.startsWith("data: ")) data += line.slice(6);
      }
      if (data) completed = handleEvent(eventName, JSON.parse(data) as SyncProgress | SyncErrorEvent) || completed;
    };
    while (true) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
      const blocks = buffer.split(/\r?\n\r?\n/);
      buffer = blocks.pop() ?? "";
      for (const block of blocks) consumeBlock(block);
      if (done) break;
    }
    if (buffer.trim()) consumeBlock(buffer);
    if (!completed) throw new Error("Strava sync ended before completion");
  };

  form.addEventListener("submit", async (event: SubmitEvent) => {
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
