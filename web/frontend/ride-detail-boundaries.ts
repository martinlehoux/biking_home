import { climbMetrics, formatDistance, formatElevation } from "./ride-detail-logic.js";
import { requireElement } from "./dom.js";
import type { BoundarySource, BoundaryTarget, ClimbBounds, RideProfilePoint } from "./types.js";

interface ActiveBoundary {
  form: HTMLFormElement;
  target: BoundaryTarget;
  source: BoundarySource;
}

interface RideBoundaryControllerOptions {
  forms: HTMLFormElement[];
  points: RideProfilePoint[];
  climbBounds: ClimbBounds[];
  onBoundaryChanged: (index: number) => void;
  onPointSelected: (index: number) => void;
  setMessage: (message: string) => void;
}

export class RideBoundaryController {
  private readonly forms: HTMLFormElement[];
  private readonly points: RideProfilePoint[];
  private readonly climbBounds: ClimbBounds[];
  private readonly onBoundaryChanged: (index: number) => void;
  private readonly onPointSelected: (index: number) => void;
  private readonly setMessage: (message: string) => void;
  private activeBoundary: ActiveBoundary | null;

  constructor({ forms, points, climbBounds, onBoundaryChanged, onPointSelected, setMessage }: RideBoundaryControllerOptions) {
    this.forms = forms;
    this.points = points;
    this.climbBounds = climbBounds;
    this.onBoundaryChanged = onBoundaryChanged;
    this.onPointSelected = onPointSelected;
    this.setMessage = setMessage;
    this.activeBoundary = null;
    this.initialize();
  }

  initialize(): void {
    for (const form of this.forms) {
      const item = form.closest<HTMLElement>("[data-climb-item]");
      if (!item) continue;
      const climbIndex = Number.parseInt(item.dataset.climbIndex ?? "", 10);
      this.climbBounds[climbIndex] = {
        startIndex: Number.parseInt(
          requireElement(form.querySelector<HTMLInputElement>('[data-boundary-input="start"]'), "start boundary input").value,
          10,
        ),
        endIndex: Number.parseInt(
          requireElement(form.querySelector<HTMLInputElement>('[data-boundary-input="end"]'), "end boundary input").value,
          10,
        ),
      };
      this.updatePreview(form);
      const buttons = [...form.querySelectorAll<HTMLElement>("[data-boundary-button]")];
      for (const button of buttons) {
        button.addEventListener("click", () => {
          this.clearSelection();
          this.activeBoundary = {
            form,
            target: button.dataset.boundaryButton as BoundaryTarget,
            source: button.dataset.boundarySource as BoundarySource,
          };
          button.classList.add("active");
          const surface = this.activeBoundary.source === "map" ? "map" : "profile";
          this.setMessage(`Click the ${surface} to select the ${this.activeBoundary.target} point.`);
        });
      }
    }
  }

  isSelecting(source: BoundarySource): boolean {
    return this.activeBoundary?.source === source;
  }

  clearSelection(): void {
    this.activeBoundary = null;
    for (const form of this.forms) {
      for (const button of form.querySelectorAll("[data-boundary-button]")) button.classList.remove("active");
    }
  }

  choosePoint(index: number): void {
    if (!this.activeBoundary) return;
    const { form, target } = this.activeBoundary;
    const item = form.closest<HTMLElement>("[data-climb-item]");
    if (!item) return;
    const climbIndex = Number.parseInt(item.dataset.climbIndex ?? "", 10);
    const input = requireElement(form.querySelector<HTMLInputElement>(`[data-boundary-input="${target}"]`), `${target} boundary input`);
    const output = requireElement(form.querySelector<HTMLOutputElement>(`[data-boundary-output="${target}"]`), `${target} boundary output`);
    input.value = String(index);
    output.textContent = this.boundaryLabel(index);
    if (target === "start") this.climbBounds[climbIndex].startIndex = index;
    else this.climbBounds[climbIndex].endIndex = index;
    this.updatePreview(form);
    this.onBoundaryChanged(climbIndex);
    this.clearSelection();
    this.onPointSelected(index);
  }

  updatePreview(form: HTMLFormElement): void {
    const preview = requireElement(form.querySelector<HTMLElement>("[data-boundary-preview]"), "boundary preview");
    const item = form.closest<HTMLElement>("[data-climb-item]");
    if (!item) return;
    const climbIndex = Number.parseInt(item.dataset.climbIndex ?? "", 10);
    const metrics = climbMetrics(this.points, this.climbBounds[climbIndex]);
    if (!metrics) {
      preview.textContent = "Choose an end point after the start point.";
      return;
    }
    const summary = requireElement(item.querySelector<HTMLElement>("[data-climb-summary]"), "climb summary");
    const metricsOutput = requireElement(item.querySelector<HTMLElement>("[data-climb-metrics]"), "climb metrics");
    summary.textContent = `${formatDistance(metrics.start.distanceKm)}–${formatDistance(metrics.end.distanceKm)}`;
    metricsOutput.textContent = `${metrics.category} · ${formatDistance(metrics.distanceKm)} at ${metrics.slope.toFixed(1)}% · Cotacol ${metrics.cotacol.toFixed(1)}`;
    preview.textContent = `Preview: ${formatDistance(metrics.distanceKm)} · ${metrics.elevationGain >= 0 ? "+" : ""}${formatElevation(metrics.elevationGain)} · ${metrics.slope.toFixed(1)}% · Cotacol ${metrics.cotacol.toFixed(1)}`;
  }

  boundaryLabel(index: number): string {
    const point = this.points[index];
    return `${formatDistance(point.distanceKm)} · ${formatElevation(point.elevationM)}`;
  }
}
