import test from "node:test";
import assert from "node:assert/strict";
import {
  categoryForCotacol,
  climbMetrics,
  cotacolForClimb,
  elevationAtDistance,
  formatClimbLabel,
  formatClimbBoundaryLabel,
  formatDistance,
  formatElevation,
  nearestPointIndex,
} from "./ride-detail-logic.ts";

const points = [
  { distanceKm: 0, elevationM: 100 },
  { distanceKm: 0.12, elevationM: 106 },
  { distanceKm: 0.2, elevationM: 114 },
  { distanceKm: 0.25, elevationM: 120 },
];

test("finds the nearest profile point by distance", () => {
  assert.equal(nearestPointIndex(points, 0.01), 0);
  assert.equal(nearestPointIndex(points, 0.15), 1);
  assert.equal(nearestPointIndex(points, 0.24), 3);
});

test("interpolates elevation between route points", () => {
  assert.equal(elevationAtDistance(points, 0, 100), 105);
  assert.equal(elevationAtDistance(points, 2, 250), 120);
});

test("calculates climb metrics with Cotacol", () => {
  const metrics = climbMetrics(points, { startIndex: 0, endIndex: 3 });
  assert.ok(metrics);

  assert.equal(metrics.distanceKm, 0.25);
  assert.equal(metrics.elevationGain, 20);
  assert.equal(metrics.slope, 8);
  assert.equal(metrics.cotacol, cotacolForClimb(points, 0, 3));
  assert.equal(metrics.category, categoryForCotacol(metrics.cotacol));
});

test("formats profile climb labels with category", () => {
  assert.equal(formatClimbLabel({ name: "", category: "Cat 4", cotacol: 45.9, kind: "detected" }), "Cat 4 45.9");
  assert.equal(formatClimbLabel({ name: "Col de Test", category: "Cat 4", cotacol: 45.9, kind: "detected" }), "Col de Test");
  assert.equal(formatClimbLabel({ name: "Ventoux", category: "HC", cotacol: 600, kind: "official" }), "Ventoux (HC)");
});

test("formats climb boundary labels", () => {
  assert.equal(formatClimbBoundaryLabel(0, "start"), "Climb 1 start");
  assert.equal(formatClimbBoundaryLabel(2, "end"), "Climb 3 end");
});

test("formats profile labels consistently", () => {
  assert.equal(formatDistance(2.4), "2.4 km");
  assert.equal(formatDistance(12.4), "12 km");
  assert.equal(formatElevation(123.6), "124 m");
});
