import { clamp, formatClimbBoundaryLabel } from "./ride-detail-logic.js";
import type { ClimbBounds, RideMapColors, RideMapPass, RideProfilePoint, RideRoute } from "./types.js";
import type { CircleMarker, DivIcon, LeafletMouseEvent, Map as LeafletMap, Marker, Polyline } from "leaflet";

type LeafletApi = typeof import("leaflet");
type ClimbBoundaryMarkers = { start: Marker; end: Marker };

interface RideDetailMapOptions {
  leaflet: LeafletApi;
  element: HTMLElement;
  route: RideRoute;
  points: RideProfilePoint[];
  climbs: ClimbBounds[];
  passes: RideMapPass[];
  colors: RideMapColors;
}

export class RideDetailMap {
  private readonly leaflet: LeafletApi;
  private readonly points: RideProfilePoint[];
  private readonly colors: RideMapColors;
  private readonly map: LeafletMap;
  private readonly climbLayers: (Polyline | null)[];
  private readonly climbBoundaryMarkers: (ClimbBoundaryMarkers | null)[];
  private readonly routeCursor: CircleMarker;

  constructor({ leaflet, element, route, points, climbs, passes, colors }: RideDetailMapOptions) {
    this.leaflet = leaflet;
    this.points = points;
    this.colors = colors;
    this.map = leaflet.map(element);
    const tiles = leaflet.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    });
    tiles.addTo(this.map);
    const routeLayer = leaflet
      .geoJSON(route, {
        style: { color: colors.accent, weight: 4, opacity: 0.9 },
      })
      .addTo(this.map);
    const bounds = routeLayer.getBounds();
    const passIcon = leaflet.divIcon({
      className: "ride-map-pass-icon",
      html: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 19 10.5 6l3.25 5.5L16 8l5 11H3Z" fill="currentColor"/></svg>',
      iconSize: [28, 28],
      iconAnchor: [14, 14],
    });
    for (const pass of passes) {
      const marker = leaflet.marker([pass.latitude, pass.longitude], { icon: passIcon, alt: pass.name || "Mountain pass" }).addTo(this.map);
      const tooltip = document.createElement("span");
      tooltip.textContent = `${pass.name || "Mountain pass"} | ${Math.round(pass.elevationM)} m`;
      marker.bindTooltip(tooltip);
      bounds.extend(marker.getLatLng());
    }
    if (bounds.isValid()) this.map.fitBounds(bounds, { padding: [24, 24], maxZoom: 15 });
    this.climbLayers = climbs.map((climb) => this.createClimbLayer(climb));
    const climbStartIcon = leaflet.divIcon({
      className: "ride-map-climb-start-icon",
      html: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="6" fill="currentColor"/></svg>',
      iconSize: [26, 26],
      iconAnchor: [13, 13],
    });
    const climbEndIcon = leaflet.divIcon({
      className: "ride-map-climb-end-icon",
      html: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 3v18" stroke="currentColor" stroke-width="2"/><path d="M6 4h13v8H6Z" fill="currentColor"/><path d="M6 4h3.25v4H6ZM12.5 4h3.25v4H12.5ZM9.25 8h3.25v4H9.25ZM15.75 8H19v4h-3.25Z" fill="var(--color-forest)"/></svg>',
      iconSize: [26, 26],
      iconAnchor: [13, 13],
    });
    this.climbBoundaryMarkers = climbs.map((climb, index) => this.createClimbBoundaryMarkers(index, climb, climbStartIcon, climbEndIcon));
    this.routeCursor = leaflet
      .circleMarker([points[0].latitude, points[0].longitude], {
        color: colors.forest,
        fillColor: colors.accent,
        fillOpacity: 0,
        opacity: 0,
        radius: 7,
        weight: 3,
        interactive: false,
      })
      .addTo(this.map);
  }

  createClimbBoundaryMarkers(index: number, bounds: ClimbBounds, startIcon: DivIcon, endIcon: DivIcon): ClimbBoundaryMarkers {
    const markers = {
      start: this.createClimbBoundaryMarker(startIcon, formatClimbBoundaryLabel(index, "start"), bounds.startIndex),
      end: this.createClimbBoundaryMarker(endIcon, formatClimbBoundaryLabel(index, "end"), bounds.endIndex),
    };
    return markers;
  }

  createClimbBoundaryMarker(icon: DivIcon, label: string, pointIndex: number): Marker {
    const marker = this.leaflet.marker([0, 0], { icon, alt: label });
    const tooltip = document.createElement("span");
    tooltip.textContent = label;
    marker.bindTooltip(tooltip);
    this.updateBoundaryMarker(marker, pointIndex);
    return marker;
  }

  updateBoundaryMarker(marker: Marker, pointIndex: number | undefined): void {
    if (!this.isValidPointIndex(pointIndex)) {
      marker.remove();
      return;
    }
    const point = this.points[pointIndex];
    marker.setLatLng([point.latitude, point.longitude]);
    if (!this.map.hasLayer(marker)) marker.addTo(this.map);
  }

  isValidPointIndex(index: number | undefined): index is number {
    return typeof index === "number" && Number.isInteger(index) && index >= 0 && index < this.points.length;
  }

  createClimbLayer(climb: ClimbBounds): Polyline | null {
    if (!this.isValidBounds(climb)) return null;
    const coordinates: [number, number][] = this.points
      .slice(climb.startIndex, climb.endIndex + 1)
      .map((point) => [point.latitude, point.longitude]);
    return this.leaflet
      .polyline(coordinates, {
        color: this.colors.climbRoute,
        weight: 7,
        opacity: 0.65,
        lineCap: "round",
        lineJoin: "round",
        interactive: false,
      })
      .addTo(this.map);
  }

  isValidBounds(bounds: ClimbBounds | undefined): boolean {
    return Boolean(
      bounds &&
        Number.isInteger(bounds.startIndex) &&
        Number.isInteger(bounds.endIndex) &&
        bounds.startIndex >= 0 &&
        bounds.endIndex < this.points.length &&
        bounds.startIndex < bounds.endIndex,
    );
  }

  updateClimbLayer(index: number, bounds: ClimbBounds | undefined, active: boolean): void {
    const markers = this.climbBoundaryMarkers[index];
    if (markers) {
      this.updateBoundaryMarker(markers.start, bounds?.startIndex);
      this.updateBoundaryMarker(markers.end, bounds?.endIndex);
    }
    const layer = this.climbLayers[index];
    if (!layer) return;
    if (!bounds || !this.isValidBounds(bounds)) {
      layer.setLatLngs([]);
      return;
    }
    layer.setLatLngs(
      this.points.slice(bounds.startIndex, bounds.endIndex + 1).map((point) => [point.latitude, point.longitude] as [number, number]),
    );
    layer.setStyle({ weight: active ? 9 : 7, opacity: active ? 1 : 0.65 });
  }

  nearestPointIndex(latitude: number, longitude: number): number {
    let nearestIndex = 0;
    let nearestDistance = Infinity;
    for (let index = 0; index < this.points.length; index++) {
      const point = this.points[index];
      const distance = this.leaflet.latLng(point.latitude, point.longitude).distanceTo([latitude, longitude]);
      if (distance < nearestDistance) {
        nearestIndex = index;
        nearestDistance = distance;
      }
    }
    return nearestIndex;
  }

  showPoint(index: number): void {
    const point = this.points[clamp(index, 0, this.points.length - 1)];
    this.routeCursor.setLatLng([point.latitude, point.longitude]);
    this.routeCursor.setStyle({ opacity: 1, fillOpacity: 0.9 });
  }

  clearPoint() {
    this.routeCursor.setStyle({ opacity: 0, fillOpacity: 0 });
  }

  zoomToClimb(bounds: ClimbBounds | undefined): void {
    if (!bounds || !this.isValidBounds(bounds)) return;
    const climbPoints = this.points.slice(bounds.startIndex, bounds.endIndex + 1);
    const mapBounds = this.leaflet.latLngBounds(climbPoints.map((point) => [point.latitude, point.longitude] as [number, number]));
    if (mapBounds.isValid()) this.map.fitBounds(mapBounds, { padding: [32, 32], maxZoom: 15 });
  }

  onClick(listener: (event: LeafletMouseEvent) => void): void {
    this.map.on("click", listener);
  }
}
