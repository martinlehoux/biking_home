import { OfficialClimbProfileController } from "./official-climb-profile.js";
import { RideBoundaryController } from "./ride-detail-boundaries.js";
import { RideProfileCanvas } from "./ride-detail-canvas.js";
import { RideDetailMap } from "./ride-detail-map.js";
import { formatDistance, formatElevation } from "./ride-detail-logic.js";
import type { ClimbBounds, RideMapColors, RideProfile, RideProfileColors, RideProfilePoint, RideRoute } from "./types.js";

export const mountRideDetail = (): void => {
  const mapElement = document.getElementById("ride-map");
  const routeElement = document.getElementById("ride-route");
  const profileElement = document.getElementById("ride-profile");
  const canvas = document.getElementById("ride-profile-chart");
  const hoverOutput = document.getElementById("ride-profile-hover");
  if (!mapElement || !routeElement || !profileElement || !canvas || !hoverOutput) return;
  const routeScript = routeElement as HTMLScriptElement;
  const profileScript = profileElement as HTMLScriptElement;
  const profileCanvasElement = canvas as HTMLCanvasElement;
  const leaflet = window.L;
  if (!leaflet) return;

  const colorProbe = document.createElement("span");
  colorProbe.hidden = true;
  document.body.append(colorProbe);
  const resolveColor = (name: string): string => {
    colorProbe.style.color = `var(${name})`;
    return getComputedStyle(colorProbe).color;
  };
  const profileColors: RideProfileColors = {
    subtle: resolveColor("--color-subtle"),
    plotSurface: resolveColor("--color-plot-surface"),
    plotSurfaceOverlay: resolveColor("--color-plot-surface-overlay"),
    grid: resolveColor("--color-plot-grid"),
    profileFill: resolveColor("--color-profile-fill"),
    profileLine: resolveColor("--color-profile-line"),
    accentFill: resolveColor("--color-accent-fill"),
    climbLabel: resolveColor("--color-climb-label"),
    crossing: resolveColor("--color-crossing"),
    crossingLabel: resolveColor("--color-crossing-label"),
    hoverLine: resolveColor("--color-hover-line"),
    forest: resolveColor("--color-forest"),
    climbFocusFill: resolveColor("--color-climb-focus-fill"),
  };
  const mapColors: RideMapColors = {
    accent: resolveColor("--color-accent"),
    forest: resolveColor("--color-forest"),
    climbRoute: resolveColor("--color-climb-route"),
  };
  colorProbe.remove();

  const route = JSON.parse(routeScript.textContent ?? "") as RideRoute;
  const profile = JSON.parse(profileScript.textContent ?? "") as RideProfile;
  const points: RideProfilePoint[] = profile.points;
  if (points.length === 0) return;

  const climbItems = [...document.querySelectorAll<HTMLElement>("[data-climb-item]")];
  const climbItemIndices = climbItems.map((item) => Number.parseInt(item.dataset.climbIndex ?? "", 10));
  const previousClimbButton = document.querySelector<HTMLButtonElement>("[data-climb-previous]");
  const nextClimbButton = document.querySelector<HTMLButtonElement>("[data-climb-next]");
  const climbPosition = document.querySelector<HTMLElement>("[data-climb-position]");
  const climbBounds: ClimbBounds[] = profile.climbs.map((climb) => ({ startIndex: climb.startIndex, endIndex: climb.endIndex }));
  const state = {
    focusedClimbItemIndex: 0,
    focusedClimbIndex: climbItemIndices[0] ?? 0,
  };

  const mapController = new RideDetailMap({
    leaflet,
    element: mapElement,
    route,
    points,
    climbs: climbBounds,
    colors: mapColors,
  });
  const officialProfileController = new OfficialClimbProfileController(points);
  let boundaryController: RideBoundaryController | undefined;
  const profileCanvas = new RideProfileCanvas({
    canvas: profileCanvasElement,
    points,
    profile,
    colors: profileColors,
    getClimbBounds: () => climbBounds,
    canSelectPoint: (source) => boundaryController?.isSelecting(source) ?? false,
    onPointSelected: (index) => boundaryController?.choosePoint(index),
    onPointHover: (index) => {
      if (index < 0) {
        mapController.clearPoint();
        hoverOutput.textContent = "Hover or focus the profile to inspect elevation.";
        return;
      }
      const point = points[index];
      mapController.showPoint(index);
      hoverOutput.textContent = `${formatDistance(point.distanceKm)} · ${formatElevation(point.elevationM)}`;
    },
  });

  const updateClimbLayer = (index: number): void => {
    mapController.updateClimbLayer(index, climbBounds[index], index === state.focusedClimbIndex);
  };
  boundaryController = new RideBoundaryController({
    forms: [...document.querySelectorAll<HTMLFormElement>("[data-official-climb-form]")],
    points,
    climbBounds,
    onBoundaryChanged: (index) => {
      updateClimbLayer(index);
      profileCanvas.redraw();
    },
    onPointSelected: (index) => profileCanvas.showPoint(index),
    setMessage: (message) => {
      hoverOutput.textContent = message;
    },
  });

  mapController.onClick((event) => {
    if (!boundaryController?.isSelecting("map")) return;
    boundaryController.choosePoint(mapController.nearestPointIndex(event.latlng.lat, event.latlng.lng));
  });

  const updateClimbFocus = (index: number, zoom: boolean): void => {
    if (climbItems.length === 0) return;
    state.focusedClimbItemIndex = Math.max(0, Math.min(climbItems.length - 1, index));
    state.focusedClimbIndex = climbItemIndices[state.focusedClimbItemIndex];
    for (let itemIndex = 0; itemIndex < climbItems.length; itemIndex++) {
      const active = itemIndex === state.focusedClimbItemIndex;
      climbItems[itemIndex].hidden = !active;
      climbItems[itemIndex].setAttribute("aria-hidden", String(!active));
      climbItems[itemIndex].classList.toggle("focused", active);
      updateClimbLayer(climbItemIndices[itemIndex]);
    }
    if (climbPosition) climbPosition.textContent = `Climb ${state.focusedClimbItemIndex + 1} of ${climbItems.length}`;
    if (previousClimbButton) previousClimbButton.disabled = state.focusedClimbItemIndex === 0;
    if (nextClimbButton) nextClimbButton.disabled = state.focusedClimbItemIndex === climbItems.length - 1;
    boundaryController?.clearSelection();
    profileCanvas.setFocusedClimbIndex(state.focusedClimbIndex);
    if (zoom) mapController.zoomToClimb(climbBounds[state.focusedClimbIndex]);
  };

  if (previousClimbButton) previousClimbButton.addEventListener("click", () => updateClimbFocus(state.focusedClimbItemIndex - 1, true));
  if (nextClimbButton) nextClimbButton.addEventListener("click", () => updateClimbFocus(state.focusedClimbItemIndex + 1, true));
  window.addEventListener("resize", () => {
    profileCanvas.redraw();
    officialProfileController.redrawOpen();
  });
  if (climbItems.length > 0) updateClimbFocus(0, false);
  else profileCanvas.redraw();
};

mountRideDetail();
