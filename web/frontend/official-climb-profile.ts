import { displayStepForLength, officialProfileSections } from "./official-climb-profile-logic.js";
import type { OfficialProfileColors, RideProfilePoint } from "./types.js";

export class OfficialClimbProfileController {
  readonly points: RideProfilePoint[];
  readonly colors: OfficialProfileColors;
  readonly cards: HTMLDetailsElement[];

  constructor(points: RideProfilePoint[]) {
    this.points = points;
    this.colors = this.resolveColors();
    this.cards = [...document.querySelectorAll("[data-official-climb-card]")].map((card) => card as HTMLDetailsElement);
    for (const card of this.cards) {
      card.addEventListener("toggle", () => {
        if (!card.open) return;
        for (const other of this.cards) {
          if (other !== card) other.open = false;
        }
        const profileCanvas = card.querySelector<HTMLCanvasElement>("[data-official-profile]");
        if (profileCanvas) this.drawOfficialProfile(profileCanvas);
      });
    }
  }

  resolveColors(): OfficialProfileColors {
    const colorProbe = document.createElement("span");
    colorProbe.hidden = true;
    document.body.append(colorProbe);
    const resolveColor = (name: string): string => {
      colorProbe.style.color = `var(${name})`;
      return getComputedStyle(colorProbe).color;
    };
    const colors = {
      downhill: resolveColor("--color-profile-downhill"),
      "0-3": resolveColor("--color-profile-0-3"),
      "3-6": resolveColor("--color-profile-3-6"),
      "6-9": resolveColor("--color-profile-6-9"),
      "9-plus": resolveColor("--color-profile-9-plus"),
      plotSurface: resolveColor("--color-plot-surface"),
      grid: resolveColor("--color-plot-grid"),
      xGrid: resolveColor("--color-profile-grid-x"),
      yGrid: resolveColor("--color-profile-grid-y"),
      subtle: resolveColor("--color-subtle"),
      accent: resolveColor("--color-accent"),
    };
    colorProbe.remove();
    return colors;
  }

  drawOfficialProfile(profileCanvas: HTMLCanvasElement): void {
    const { points, colors } = this;
    const startIndex = Number.parseInt(profileCanvas.dataset.profileStart ?? "", 10);
    const endIndex = Number.parseInt(profileCanvas.dataset.profileEnd ?? "", 10);
    if (
      !Number.isInteger(startIndex) ||
      !Number.isInteger(endIndex) ||
      startIndex < 0 ||
      endIndex >= points.length ||
      startIndex >= endIndex
    )
      return;
    const rect = profileCanvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    const context = profileCanvas.getContext("2d");
    if (!context) return;
    const ratio = window.devicePixelRatio || 1;
    profileCanvas.width = Math.floor(rect.width * ratio);
    profileCanvas.height = Math.floor(rect.height * ratio);
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    const plot = { left: 10, right: Math.max(11, rect.width - 44), top: 16, bottom: rect.height - 34 };
    const climbLengthM = (points[endIndex].distanceKm - points[startIndex].distanceKm) * 1000;
    const sections = officialProfileSections(points, startIndex, endIndex, displayStepForLength(climbLengthM));
    if (sections.length === 0) return;
    let minElevation = sections[0].startElevation;
    let maxElevation = minElevation;
    for (const section of sections) {
      minElevation = Math.min(minElevation, section.startElevation, section.endElevation);
      maxElevation = Math.max(maxElevation, section.startElevation, section.endElevation);
    }
    const elevationPadding = Math.max((maxElevation - minElevation) * 0.12, 8);
    minElevation -= elevationPadding;
    maxElevation += elevationPadding;
    const minDistance = sections[0].startDistanceKm;
    const maxDistance = sections[sections.length - 1].endDistanceKm;
    const distanceSpan = Math.max(maxDistance - minDistance, 0.1);
    const elevationSpan = Math.max(maxElevation - minElevation, 1);
    const xForDistance = (distanceKm: number) => plot.left + ((distanceKm - minDistance) / distanceSpan) * (plot.right - plot.left);
    const yForElevation = (elevationM: number) => plot.bottom - ((elevationM - minElevation) / elevationSpan) * (plot.bottom - plot.top);
    context.clearRect(0, 0, rect.width, rect.height);
    context.fillStyle = colors.plotSurface;
    context.fillRect(0, 0, rect.width, rect.height);
    context.font = "9px system-ui, sans-serif";
    context.lineWidth = 1;
    context.setLineDash([4, 4]);
    context.strokeStyle = colors.yGrid;
    const yGridElevations: number[] = [];
    for (let elevationM = Math.ceil(minElevation / 100) * 100; elevationM <= maxElevation; elevationM += 100) {
      yGridElevations.push(elevationM);
    }
    context.save();
    context.beginPath();
    context.moveTo(plot.left, plot.bottom);
    context.lineTo(plot.left, yForElevation(sections[0].startElevation));
    for (const section of sections) {
      context.lineTo(xForDistance(section.endDistanceKm), yForElevation(section.endElevation));
    }
    context.lineTo(plot.right, plot.bottom);
    context.closePath();
    context.clip();
    for (const elevationM of yGridElevations) {
      const y = yForElevation(elevationM);
      context.beginPath();
      context.moveTo(plot.left, y);
      context.lineTo(plot.right, y);
      context.stroke();
    }
    context.restore();
    context.fillStyle = colors.yGrid;
    context.textAlign = "left";
    context.textBaseline = "middle";
    for (const elevationM of yGridElevations) {
      context.fillText(`${elevationM} m`, plot.right + 6, yForElevation(elevationM));
    }
    context.setLineDash([]);
    context.strokeStyle = colors.grid;
    context.beginPath();
    context.moveTo(plot.left, plot.bottom);
    context.lineTo(plot.right, plot.bottom);
    context.stroke();
    for (const section of sections) {
      const startX = xForDistance(section.startDistanceKm);
      const endX = xForDistance(section.endDistanceKm);
      context.beginPath();
      context.moveTo(startX, plot.bottom);
      context.lineTo(startX, yForElevation(section.startElevation));
      context.lineTo(endX, yForElevation(section.endElevation));
      context.lineTo(endX, plot.bottom);
      context.closePath();
      context.globalAlpha = 0.25;
      context.fillStyle = colors[section.band];
      context.fill();
      context.globalAlpha = 1;
      context.beginPath();
      context.moveTo(startX, yForElevation(section.startElevation));
      context.lineTo(endX, yForElevation(section.endElevation));
      context.strokeStyle = colors[section.band];
      context.lineWidth = 2.5;
      context.stroke();
    }
    context.save();
    context.beginPath();
    context.moveTo(plot.left, plot.bottom);
    context.lineTo(plot.left, yForElevation(sections[0].startElevation));
    for (const section of sections) {
      context.lineTo(xForDistance(section.endDistanceKm), yForElevation(section.endElevation));
    }
    context.lineTo(plot.right, plot.bottom);
    context.closePath();
    context.clip();
    context.strokeStyle = colors.xGrid;
    context.lineWidth = 1;
    context.setLineDash([4, 4]);
    for (let sectionIndex = 0; sectionIndex <= sections.length; sectionIndex++) {
      const distanceKm = sectionIndex === sections.length ? maxDistance : sections[sectionIndex].startDistanceKm;
      const x = xForDistance(distanceKm);
      context.beginPath();
      context.moveTo(x, plot.top);
      context.lineTo(x, plot.bottom);
      context.stroke();
    }
    context.restore();
    context.setLineDash([]);
    context.font = "9px system-ui, sans-serif";
    context.fillStyle = colors.subtle;
    context.textBaseline = "bottom";
    context.textAlign = "center";
    context.save();
    context.beginPath();
    context.rect(plot.left, plot.top, plot.right - plot.left, plot.bottom - plot.top);
    context.clip();
    for (const section of sections) {
      const startX = xForDistance(section.startDistanceKm);
      const endX = xForDistance(section.endDistanceKm);
      const label = `${section.slopePercent.toFixed(1)}%`;
      context.fillText(label, (startX + endX) / 2, plot.bottom - 4);
    }
    context.restore();
    let topElevation = sections[0].startElevation;
    let topDistance = sections[0].startDistanceKm;
    for (const section of sections) {
      if (section.startElevation > topElevation) {
        topElevation = section.startElevation;
        topDistance = section.startDistanceKm;
      }
      if (section.endElevation > topElevation) {
        topElevation = section.endElevation;
        topDistance = section.endDistanceKm;
      }
    }
    context.fillStyle = colors.accent;
    context.beginPath();
    context.arc(xForDistance(topDistance), yForElevation(topElevation), 3.5, 0, 2 * Math.PI);
    context.fill();
    context.font = "9px system-ui, sans-serif";
    context.fillStyle = colors.subtle;
    context.textBaseline = "top";
    context.strokeStyle = colors.grid;
    context.lineWidth = 1;
    for (let sectionIndex = 0; sectionIndex <= sections.length; sectionIndex++) {
      const distanceKm =
        sectionIndex === sections.length ? maxDistance - minDistance : sections[sectionIndex].startDistanceKm - minDistance;
      const x = xForDistance(minDistance + distanceKm);
      context.beginPath();
      context.moveTo(x, plot.bottom);
      context.lineTo(x, plot.bottom + 3);
      context.stroke();
      context.textAlign = sectionIndex === 0 ? "left" : sectionIndex === sections.length ? "right" : "center";
      context.fillText(this.formatDistance(distanceKm), x, plot.bottom + 5);
    }
  }

  formatDistance(distance: number): string {
    return `${distance.toFixed(distance < 10 ? 1 : 0)} km`;
  }

  redrawOpen() {
    const profileCanvases = document.querySelectorAll<HTMLCanvasElement>("[data-official-profile]");
    for (const profileCanvas of profileCanvases) {
      const card = profileCanvas.closest<HTMLDetailsElement>("[data-official-climb-card]");
      if (card?.open) this.drawOfficialProfile(profileCanvas);
    }
  }
}
