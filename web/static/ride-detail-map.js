import { clamp } from "./ride-detail-logic.js";

export class RideDetailMap {
  constructor({ leaflet, element, route, points, climbs, colors }) {
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
    if (bounds.isValid()) this.map.fitBounds(bounds, { padding: [24, 24], maxZoom: 15 });
    this.climbLayers = climbs.map((climb) => this.createClimbLayer(climb));
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

  createClimbLayer(climb) {
    if (!this.isValidBounds(climb)) return null;
    const coordinates = this.points.slice(climb.startIndex, climb.endIndex + 1).map((point) => [point.latitude, point.longitude]);
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

  isValidBounds(bounds) {
    return (
      bounds &&
      Number.isInteger(bounds.startIndex) &&
      Number.isInteger(bounds.endIndex) &&
      bounds.startIndex >= 0 &&
      bounds.endIndex < this.points.length &&
      bounds.startIndex < bounds.endIndex
    );
  }

  updateClimbLayer(index, bounds, active) {
    const layer = this.climbLayers[index];
    if (!layer) return;
    if (!this.isValidBounds(bounds)) {
      layer.setLatLngs([]);
      return;
    }
    layer.setLatLngs(this.points.slice(bounds.startIndex, bounds.endIndex + 1).map((point) => [point.latitude, point.longitude]));
    layer.setStyle({ weight: active ? 9 : 7, opacity: active ? 1 : 0.65 });
  }

  nearestPointIndex(latitude, longitude) {
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

  showPoint(index) {
    const point = this.points[clamp(index, 0, this.points.length - 1)];
    this.routeCursor.setLatLng([point.latitude, point.longitude]);
    this.routeCursor.setStyle({ opacity: 1, fillOpacity: 0.9 });
  }

  clearPoint() {
    this.routeCursor.setStyle({ opacity: 0, fillOpacity: 0 });
  }

  zoomToClimb(bounds) {
    if (!this.isValidBounds(bounds)) return;
    const climbPoints = this.points.slice(bounds.startIndex, bounds.endIndex + 1);
    const mapBounds = this.leaflet.latLngBounds(climbPoints.map((point) => [point.latitude, point.longitude]));
    if (mapBounds.isValid()) this.map.fitBounds(mapBounds, { padding: [32, 32], maxZoom: 15 });
  }

  onClick(listener) {
    this.map.on("click", listener);
  }
}
