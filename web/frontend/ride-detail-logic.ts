import type { ClimbBounds, ClimbMetrics, ProfilePoint } from "./types.js";

export const clamp = (value: number, min: number, max: number): number => Math.max(min, Math.min(max, value));

export const formatDistance = (distance: number): string => `${distance.toFixed(distance < 10 ? 1 : 0)} km`;

export const formatElevation = (elevation: number): string => `${Math.round(elevation)} m`;

export const nearestPointIndex = (points: ProfilePoint[], distance: number): number => {
  let low = 0;
  let high = points.length - 1;
  while (low < high) {
    const middle = Math.floor((low + high) / 2);
    if (points[middle].distanceKm < distance) low = middle + 1;
    else high = middle;
  }
  if (low === 0) return low;
  const previous = points[low - 1];
  return distance - previous.distanceKm < points[low].distanceKm - distance ? low - 1 : low;
};

export const categoryForCotacol = (cotacol: number): string => {
  if (cotacol < 35) return "NO";
  if (cotacol < 80) return "Cat 4";
  if (cotacol < 180) return "Cat 3";
  if (cotacol < 250) return "Cat 2";
  if (cotacol < 600) return "Cat 1";
  return "HC";
};

export const formatClimbLabel = (name: string, category: string, cotacol: number): string => name || `${category} ${cotacol.toFixed(1)}`;

export const elevationAtDistance = (points: ProfilePoint[], index: number, distanceM: number): number => {
  if (index + 1 >= points.length) return points[index].elevationM;
  const startDistanceM = points[index].distanceKm * 1000;
  const endDistanceM = points[index + 1].distanceKm * 1000;
  const fraction = (distanceM - startDistanceM) / (endDistanceM - startDistanceM);
  return points[index].elevationM + fraction * (points[index + 1].elevationM - points[index].elevationM);
};

export const cotacolForClimb = (points: ProfilePoint[], startIndex: number, endIndex: number): number => {
  const startDistanceM = points[startIndex].distanceKm * 1000;
  const lastDistanceM = points[endIndex].distanceKm * 1000;
  if (lastDistanceM <= startDistanceM) return 0;
  let cotacol = 0;
  let pointIndex = startIndex;
  for (let segmentStartM = startDistanceM; segmentStartM < lastDistanceM; segmentStartM += 100) {
    const segmentEndM = Math.min(segmentStartM + 100, lastDistanceM);
    while (pointIndex < endIndex && points[pointIndex + 1].distanceKm * 1000 <= segmentStartM) pointIndex++;
    const startElevation = elevationAtDistance(points, pointIndex, segmentStartM);
    while (pointIndex < endIndex && points[pointIndex + 1].distanceKm * 1000 < segmentEndM) pointIndex++;
    const endElevation = elevationAtDistance(points, pointIndex, segmentEndM);
    const slope = (endElevation - startElevation) / (segmentEndM - segmentStartM);
    if (slope > 0) cotacol += ((segmentEndM - segmentStartM) / 1000) * (slope * 100) ** 2;
  }
  return cotacol;
};

export const climbMetrics = (points: ProfilePoint[], bounds: ClimbBounds | undefined): ClimbMetrics | null => {
  if (
    !bounds ||
    !Number.isInteger(bounds.startIndex) ||
    !Number.isInteger(bounds.endIndex) ||
    bounds.startIndex < 0 ||
    bounds.endIndex >= points.length ||
    bounds.startIndex >= bounds.endIndex
  ) {
    return null;
  }
  const start = points[bounds.startIndex];
  const end = points[bounds.endIndex];
  const distanceKm = end.distanceKm - start.distanceKm;
  const elevationGain = end.elevationM - start.elevationM;
  const slope = distanceKm > 0 ? elevationGain / (distanceKm * 10) : 0;
  const cotacol = cotacolForClimb(points, bounds.startIndex, bounds.endIndex);
  return {
    start,
    end,
    distanceKm,
    elevationGain,
    slope,
    cotacol,
    category: categoryForCotacol(cotacol),
  };
};
