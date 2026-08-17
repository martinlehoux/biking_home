import { clamp, climbMetrics, formatClimbLabel, formatDistance, formatElevation, nearestPointIndex } from "./ride-detail-logic.js";
import type { BoundarySource, ClimbBounds, RideProfile, RideProfileColors, RideProfilePoint } from "./types.js";

interface Plot {
  left: number;
  right: number;
  top: number;
  bottom: number;
}

interface RideProfileCanvasOptions {
  canvas: HTMLCanvasElement;
  points: RideProfilePoint[];
  profile: RideProfile;
  colors: RideProfileColors;
  getClimbBounds: () => ClimbBounds[];
  canSelectPoint: (source: BoundarySource) => boolean;
  onPointSelected: (index: number) => void;
  onPointHover: (index: number) => void;
}

export class RideProfileCanvas {
  private readonly canvas: HTMLCanvasElement;
  private readonly points: RideProfilePoint[];
  private readonly profile: RideProfile;
  private readonly colors: RideProfileColors;
  private readonly getClimbBounds: () => ClimbBounds[];
  private readonly canSelectPoint: (source: BoundarySource) => boolean;
  private readonly onPointSelected: (index: number) => void;
  private readonly onPointHover: (index: number) => void;
  private readonly context: CanvasRenderingContext2D;
  private hoveredIndex: number;
  private focusedClimbIndex: number;
  private plot: Plot | null;
  private readonly minDistance: number;
  private readonly maxDistance: number;
  private readonly minElevation: number;
  private readonly maxElevation: number;

  constructor({
    canvas,
    points,
    profile,
    colors,
    getClimbBounds,
    canSelectPoint,
    onPointSelected,
    onPointHover,
  }: RideProfileCanvasOptions) {
    this.canvas = canvas;
    this.points = points;
    this.profile = profile;
    this.colors = colors;
    this.getClimbBounds = getClimbBounds;
    this.canSelectPoint = canSelectPoint;
    this.onPointSelected = onPointSelected;
    this.onPointHover = onPointHover;
    const context = canvas.getContext("2d");
    if (!context) throw new Error("Ride profile canvas is unavailable");
    this.context = context;
    this.hoveredIndex = -1;
    this.focusedClimbIndex = 0;
    this.plot = null;
    this.minDistance = points[0].distanceKm;
    this.maxDistance = points[points.length - 1].distanceKm;
    let minElevation = points[0].elevationM;
    let maxElevation = points[0].elevationM;
    for (const point of points) {
      minElevation = Math.min(minElevation, point.elevationM);
      maxElevation = Math.max(maxElevation, point.elevationM);
    }
    const elevationPadding = Math.max((maxElevation - minElevation) * 0.1, 20);
    this.minElevation = minElevation - elevationPadding;
    this.maxElevation = maxElevation + elevationPadding;
    this.bindEvents();
  }

  setFocusedClimbIndex(index: number): void {
    this.focusedClimbIndex = index;
    this.draw();
  }

  redraw(): void {
    this.draw();
  }

  xForDistance(distance: number): number {
    const plot = this.plot;
    if (!plot) throw new Error("Ride profile canvas has not been drawn");
    const span = Math.max(this.maxDistance - this.minDistance, 1);
    return plot.left + ((distance - this.minDistance) / span) * (plot.right - plot.left);
  }

  yForElevation(elevation: number): number {
    const plot = this.plot;
    if (!plot) throw new Error("Ride profile canvas has not been drawn");
    const span = Math.max(this.maxElevation - this.minElevation, 1);
    return plot.bottom - ((elevation - this.minElevation) / span) * (plot.bottom - plot.top);
  }

  draw(): void {
    const rect = this.canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    const ratio = window.devicePixelRatio || 1;
    this.canvas.width = Math.floor(rect.width * ratio);
    this.canvas.height = Math.floor(rect.height * ratio);
    this.context.setTransform(ratio, 0, 0, ratio, 0, 0);
    const plot: Plot = { left: 52, right: rect.width - 20, top: 24, bottom: rect.height - 34 };
    this.plot = plot;
    this.context.clearRect(0, 0, rect.width, rect.height);
    this.context.fillStyle = this.colors.plotSurface;
    this.context.fillRect(0, 0, rect.width, rect.height);

    this.context.font = "12px system-ui, sans-serif";
    this.drawGrid(plot);
    const climbBounds = this.getClimbBounds();
    this.drawProfileArea(plot);
    this.drawClimbBands(plot, climbBounds);
    this.drawCrossingLines(plot);
    this.drawProfileLine();
    this.drawClimbLabels(plot, climbBounds);
    this.drawCrossingLabels(plot);
    this.drawDistanceLabels(plot);

    if (this.hoveredIndex >= 0) this.drawHover(plot);
  }

  private drawGrid(plot: Plot): void {
    for (let step = 0; step <= 4; step++) {
      const fraction = step / 4;
      const y = plot.top + fraction * (plot.bottom - plot.top);
      const elevation = this.maxElevation - fraction * (this.maxElevation - this.minElevation);
      this.context.strokeStyle = this.colors.grid;
      this.context.lineWidth = 1;
      this.context.beginPath();
      this.context.moveTo(plot.left, y);
      this.context.lineTo(plot.right, y);
      this.context.stroke();
      this.context.fillStyle = this.colors.subtle;
      this.context.textAlign = "right";
      this.context.fillText(formatElevation(elevation), plot.left - 8, y);
    }
  }

  private drawProfileArea(plot: Plot): void {
    this.context.beginPath();
    this.context.moveTo(this.xForDistance(this.points[0].distanceKm), plot.bottom);
    for (const point of this.points) this.context.lineTo(this.xForDistance(point.distanceKm), this.yForElevation(point.elevationM));
    this.context.lineTo(this.xForDistance(this.points[this.points.length - 1].distanceKm), plot.bottom);
    this.context.closePath();
    this.context.fillStyle = this.colors.profileFill;
    this.context.fill();
  }

  private drawClimbBands(plot: Plot, climbBounds: ClimbBounds[]): void {
    for (let climbIndex = 0; climbIndex < (this.profile.climbs || []).length; climbIndex++) {
      const metrics = climbMetrics(this.points, climbBounds[climbIndex]);
      if (!metrics) continue;
      const startX = clamp(this.xForDistance(metrics.start.distanceKm), plot.left, plot.right);
      const endX = clamp(this.xForDistance(metrics.end.distanceKm), plot.left, plot.right);
      this.context.fillStyle = climbIndex === this.focusedClimbIndex ? this.colors.climbFocusFill : this.colors.accentFill;
      this.context.fillRect(startX, plot.top, Math.max(endX - startX, 1), plot.bottom - plot.top);
    }
  }

  private drawCrossingLines(plot: Plot): void {
    for (const crossing of this.profile.crossings || []) {
      const x = this.xForDistance(crossing.distanceKm);
      this.context.strokeStyle = this.colors.crossing;
      this.context.lineWidth = 1;
      this.context.setLineDash([4, 3]);
      this.context.beginPath();
      this.context.moveTo(x, plot.top);
      this.context.lineTo(x, plot.bottom);
      this.context.stroke();
      this.context.setLineDash([]);
    }
  }

  private drawProfileLine(): void {
    this.context.beginPath();
    for (let index = 0; index < this.points.length; index++) {
      const point = this.points[index];
      if (index === 0) this.context.moveTo(this.xForDistance(point.distanceKm), this.yForElevation(point.elevationM));
      else this.context.lineTo(this.xForDistance(point.distanceKm), this.yForElevation(point.elevationM));
    }
    this.context.strokeStyle = this.colors.profileLine;
    this.context.lineWidth = 2.5;
    this.context.stroke();
  }

  private drawClimbLabels(plot: Plot, climbBounds: ClimbBounds[]): void {
    for (let climbIndex = 0; climbIndex < (this.profile.climbs || []).length; climbIndex++) {
      const climb = this.profile.climbs[climbIndex];
      const metrics = climbMetrics(this.points, climbBounds[climbIndex]);
      if (!metrics) continue;
      const startX = clamp(this.xForDistance(metrics.start.distanceKm), plot.left, plot.right);
      const endX = clamp(this.xForDistance(metrics.end.distanceKm), plot.left, plot.right);
      const label = formatClimbLabel({
        name: climb.name,
        category: metrics.category,
        cotacol: metrics.cotacol,
        kind: climb.officialClimbId !== undefined ? "official" : "detected",
      });
      this.context.fillStyle = this.colors.climbLabel;
      this.context.textAlign = "center";
      this.context.textBaseline = "middle";
      const labelWidth = this.context.measureText(label).width;
      const labelX = clamp((startX + endX) / 2, plot.left + 8, plot.right - 8);
      const labelAngle = -Math.PI / 3;
      const labelHeight = Math.abs(Math.sin(labelAngle)) * labelWidth + Math.abs(Math.cos(labelAngle)) * 14;
      const labelY = clamp(plot.top + labelHeight / 2 + 4, plot.top + labelHeight / 2, plot.bottom - labelHeight / 2);
      this.context.save();
      this.context.translate(labelX, labelY);
      this.context.rotate(labelAngle);
      this.context.fillText(label, 0, 0);
      this.context.restore();
    }
  }

  private drawCrossingLabels(plot: Plot): void {
    for (const crossing of this.profile.crossings || []) {
      const x = this.xForDistance(crossing.distanceKm);
      const label = `${crossing.name} ${Math.round(crossing.passElevationM)} m`;
      const labelWidth = this.context.measureText(label).width;
      const labelX = clamp(x, plot.left + labelWidth / 2 + 3, plot.right - labelWidth / 2 - 3);
      const labelY = plot.top - 6;
      this.context.fillStyle = this.colors.plotSurfaceOverlay;
      this.context.fillRect(labelX - labelWidth / 2 - 3, labelY - 15, labelWidth + 6, 16);
      this.context.fillStyle = this.colors.crossingLabel;
      this.context.textAlign = "center";
      this.context.textBaseline = "bottom";
      this.context.fillText(label, labelX, labelY);
    }
  }

  private drawDistanceLabels(plot: Plot): void {
    this.context.fillStyle = this.colors.subtle;
    this.context.textAlign = "center";
    this.context.textBaseline = "top";
    for (let step = 0; step <= 4; step++) {
      const distance = this.minDistance + (step / 4) * (this.maxDistance - this.minDistance);
      this.context.fillText(formatDistance(distance), this.xForDistance(distance), plot.bottom + 10);
    }
  }

  private drawHover(plot: Plot): void {
    const point = this.points[this.hoveredIndex];
    const x = this.xForDistance(point.distanceKm);
    const y = this.yForElevation(point.elevationM);
    this.context.strokeStyle = this.colors.hoverLine;
    this.context.lineWidth = 1;
    this.context.setLineDash([3, 3]);
    this.context.beginPath();
    this.context.moveTo(x, plot.top);
    this.context.lineTo(x, plot.bottom);
    this.context.stroke();
    this.context.setLineDash([]);
    this.context.fillStyle = this.colors.forest;
    this.context.beginPath();
    this.context.arc(x, y, 5, 0, 2 * Math.PI);
    this.context.fill();
  }

  clearHover(): void {
    this.hoveredIndex = -1;
    this.onPointHover(-1);
    this.draw();
  }

  showPoint(index: number): void {
    this.hoveredIndex = clamp(index, 0, this.points.length - 1);
    this.onPointHover(this.hoveredIndex);
    this.draw();
  }

  pointIndexAtX(x: number): number {
    const plot = this.plot;
    if (!plot) throw new Error("Ride profile canvas has not been drawn");
    const distance = this.minDistance + ((x - plot.left) / (plot.right - plot.left)) * (this.maxDistance - this.minDistance);
    return nearestPointIndex(this.points, distance);
  }

  bindEvents(): void {
    this.canvas.addEventListener("pointermove", (event: PointerEvent) => {
      if (!this.plot) return;
      const rect = this.canvas.getBoundingClientRect();
      const x = event.clientX - rect.left;
      if (x < this.plot.left || x > this.plot.right) {
        this.clearHover();
        return;
      }
      this.showPoint(this.pointIndexAtX(x));
    });
    this.canvas.addEventListener("pointerleave", () => this.clearHover());
    this.canvas.addEventListener("pointercancel", () => this.clearHover());
    this.canvas.addEventListener("click", (event: MouseEvent) => {
      if (!this.canSelectPoint("profile") || !this.plot) return;
      const rect = this.canvas.getBoundingClientRect();
      const x = event.clientX - rect.left;
      if (x < this.plot.left || x > this.plot.right) return;
      this.onPointSelected(this.pointIndexAtX(x));
    });
    this.canvas.addEventListener("keydown", (event: KeyboardEvent) => {
      if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
      event.preventDefault();
      const direction = event.key === "ArrowRight" ? 1 : -1;
      const index = this.hoveredIndex < 0 ? (direction > 0 ? 0 : this.points.length - 1) : this.hoveredIndex + direction;
      this.showPoint(index);
    });
  }
}
