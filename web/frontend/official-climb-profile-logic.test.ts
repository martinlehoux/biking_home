import test from "node:test";
import assert from "node:assert/strict";
import { displayStepForLength, profileBandForSlope, officialProfileSections } from "./official-climb-profile-logic.ts";

test("selects a display step that keeps long profiles readable", () => {
  assert.equal(displayStepForLength(2_400), 100);
  assert.equal(displayStepForLength(4_500), 200);
  assert.equal(displayStepForLength(7_000), 500);
  assert.equal(displayStepForLength(20_000), 1000);
});

test("assigns each slope to its color band", () => {
  assert.equal(profileBandForSlope(-0.1), "downhill");
  assert.equal(profileBandForSlope(0), "downhill");
  assert.equal(profileBandForSlope(0.1), "0-3");
  assert.equal(profileBandForSlope(2.9), "0-3");
  assert.equal(profileBandForSlope(3), "3-6");
  assert.equal(profileBandForSlope(6), "6-9");
  assert.equal(profileBandForSlope(9), "9-plus");
  assert.equal(profileBandForSlope(12), "9-plus");
});

test("interpolates profile sections at the selected step", () => {
  const points = [
    { distanceKm: 0, elevationM: 100 },
    { distanceKm: 0.12, elevationM: 106 },
    { distanceKm: 0.2, elevationM: 114 },
    { distanceKm: 0.25, elevationM: 120 },
  ];
  const sections = officialProfileSections(points, 0, 3, 100);

  assert.equal(sections.length, 3);
  assert.equal(sections[0].startDistanceKm, 0);
  assert.equal(sections[0].endDistanceKm, 0.1);
  assert.equal(sections[0].startElevation, 100);
  assert.equal(sections[0].endElevation, 105);
  assert.equal(sections[2].endDistanceKm, 0.25);
  assert.equal(sections[2].endElevation, 120);
});
