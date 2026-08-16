import { climbMetrics, formatDistance, formatElevation } from "./ride-detail-logic.js";

export class RideBoundaryController {
  constructor({ forms, points, climbBounds, onBoundaryChanged, onPointSelected, setMessage }) {
    this.forms = forms;
    this.points = points;
    this.climbBounds = climbBounds;
    this.onBoundaryChanged = onBoundaryChanged;
    this.onPointSelected = onPointSelected;
    this.setMessage = setMessage;
    this.activeBoundary = null;
    this.initialize();
  }

  initialize() {
    for (const form of this.forms) {
      const item = /** @type {HTMLElement} */ (form.closest("[data-climb-item]"));
      const climbIndex = Number.parseInt(item.dataset.climbIndex, 10);
      this.climbBounds[climbIndex] = {
        startIndex: Number.parseInt(/** @type {HTMLInputElement} */ (form.querySelector('[data-boundary-input="start"]')).value, 10),
        endIndex: Number.parseInt(/** @type {HTMLInputElement} */ (form.querySelector('[data-boundary-input="end"]')).value, 10),
      };
      this.updatePreview(form);
      const buttons = [...form.querySelectorAll("[data-boundary-button]")].map((button) => /** @type {HTMLElement} */ (button));
      for (const button of buttons) {
        button.addEventListener("click", () => {
          this.clearSelection();
          this.activeBoundary = { form, target: button.dataset.boundaryButton, source: button.dataset.boundarySource };
          button.classList.add("active");
          const surface = this.activeBoundary.source === "map" ? "map" : "profile";
          this.setMessage(`Click the ${surface} to select the ${this.activeBoundary.target} point.`);
        });
      }
    }
  }

  isSelecting(source) {
    return this.activeBoundary?.source === source;
  }

  clearSelection() {
    this.activeBoundary = null;
    for (const form of this.forms) {
      for (const button of form.querySelectorAll("[data-boundary-button]")) button.classList.remove("active");
    }
  }

  choosePoint(index) {
    if (!this.activeBoundary) return;
    const { form, target } = this.activeBoundary;
    const item = /** @type {HTMLElement} */ (form.closest("[data-climb-item]"));
    const climbIndex = Number.parseInt(item.dataset.climbIndex, 10);
    const input = /** @type {HTMLInputElement} */ (form.querySelector(`[data-boundary-input="${target}"]`));
    const output = /** @type {HTMLOutputElement} */ (form.querySelector(`[data-boundary-output="${target}"]`));
    input.value = index;
    output.textContent = this.boundaryLabel(index);
    this.climbBounds[climbIndex][`${target}Index`] = index;
    this.updatePreview(form);
    this.onBoundaryChanged(climbIndex);
    this.clearSelection();
    this.onPointSelected(index);
  }

  updatePreview(form) {
    const preview = /** @type {HTMLElement} */ (form.querySelector("[data-boundary-preview]"));
    const item = /** @type {HTMLElement} */ (form.closest("[data-climb-item]"));
    const climbIndex = Number.parseInt(item.dataset.climbIndex, 10);
    const metrics = climbMetrics(this.points, this.climbBounds[climbIndex]);
    if (!metrics) {
      preview.textContent = "Choose an end point after the start point.";
      return;
    }
    const summary = /** @type {HTMLElement} */ (item.querySelector("[data-climb-summary]"));
    const metricsOutput = /** @type {HTMLElement} */ (item.querySelector("[data-climb-metrics]"));
    summary.textContent = `${formatDistance(metrics.start.distanceKm)}–${formatDistance(metrics.end.distanceKm)}`;
    metricsOutput.textContent = `${metrics.category} · ${formatDistance(metrics.distanceKm)} at ${metrics.slope.toFixed(1)}% · Cotacol ${metrics.cotacol.toFixed(1)}`;
    preview.textContent = `Preview: ${formatDistance(metrics.distanceKm)} · ${metrics.elevationGain >= 0 ? "+" : ""}${formatElevation(metrics.elevationGain)} · ${metrics.slope.toFixed(1)}% · Cotacol ${metrics.cotacol.toFixed(1)}`;
  }

  boundaryLabel(index) {
    const point = this.points[index];
    return `${formatDistance(point.distanceKm)} · ${formatElevation(point.elevationM)}`;
  }
}
