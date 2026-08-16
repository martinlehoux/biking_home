(() => {
  const mapElement = document.getElementById("ride-map");
  const routeElement = document.getElementById("ride-route");
  const profileElement = document.getElementById("ride-profile");
  const canvas = document.getElementById("ride-profile-chart");
  const hoverOutput = document.getElementById("ride-profile-hover");
  if (!mapElement || !routeElement || !profileElement || !canvas || !hoverOutput) return;
  const colorProbe = document.createElement("span");
  colorProbe.hidden = true;
  document.body.append(colorProbe);
  const resolveColor = (name) => {
    colorProbe.style.color = `var(${name})`;
    return getComputedStyle(colorProbe).color;
  };
  const colors = {
    accent: resolveColor("--color-accent"),
    forest: resolveColor("--color-forest"),
    subtle: resolveColor("--color-subtle"),
    plotSurface: resolveColor("--color-plot-surface"),
    plotSurfaceOverlay: resolveColor("--color-plot-surface-overlay"),
    grid: resolveColor("--color-plot-grid"),
    accentFill: resolveColor("--color-accent-fill"),
    climbLabel: resolveColor("--color-climb-label"),
    crossing: resolveColor("--color-crossing"),
    crossingLabel: resolveColor("--color-crossing-label"),
    hoverLine: resolveColor("--color-hover-line"),
    climbRoute: resolveColor("--color-climb-route"),
    climbFocusFill: resolveColor("--color-climb-focus-fill"),
  };
  colorProbe.remove();
  const route = JSON.parse(routeElement.textContent);
  const profile = JSON.parse(profileElement.textContent);
  const map = L.map(mapElement);
  const tiles = L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
    maxZoom: 19,
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
  });
  tiles.addTo(map);
  const routeLayer = L.geoJSON(route, {
    style: { color: colors.accent, weight: 4, opacity: 0.9 },
  }).addTo(map);
  const bounds = routeLayer.getBounds();
  if (bounds.isValid()) {
    map.fitBounds(bounds, { padding: [24, 24], maxZoom: 15 });
  }

  const points = profile.points || [];
  if (points.length === 0) return;
  const climbItems = [...document.querySelectorAll("[data-climb-item]")];
  const climbItemIndices = climbItems.map((item) => Number.parseInt(item.dataset.climbIndex, 10));
  const previousClimbButton = document.querySelector("[data-climb-previous]");
  const nextClimbButton = document.querySelector("[data-climb-next]");
  const climbPosition = document.querySelector("[data-climb-position]");
  const climbLayers = [];
  for (const climb of profile.climbs || []) {
    if (!Number.isInteger(climb.startIndex) || !Number.isInteger(climb.endIndex) || climb.startIndex < 0 || climb.endIndex >= points.length || climb.startIndex >= climb.endIndex) {
      climbLayers.push(null);
      continue;
    }
    const coordinates = points.slice(climb.startIndex, climb.endIndex + 1).map((point) => [point.latitude, point.longitude]);
    climbLayers.push(L.polyline(coordinates, { color: colors.climbRoute, weight: 7, opacity: 0.65, lineCap: "round", lineJoin: "round", interactive: false }).addTo(map));
  }
  const routeCursor = L.circleMarker([points[0].latitude, points[0].longitude], {
    color: colors.forest,
    fillColor: colors.accent,
    fillOpacity: 0,
    opacity: 0,
    radius: 7,
    weight: 3,
    interactive: false,
  }).addTo(map);

  const context = canvas.getContext("2d");
  if (!context) return;
  const state = {
    hoveredIndex: -1,
    focusedClimbItemIndex: 0,
    focusedClimbIndex: climbItemIndices[0] ?? 0,
    climbBounds: (profile.climbs || []).map((climb) => ({ startIndex: climb.startIndex, endIndex: climb.endIndex })),
    plot: null,
    width: 0,
    height: 0,
  };
  const minDistance = points[0].distanceKm;
  const maxDistance = points[points.length - 1].distanceKm;
  let minElevation = points[0].elevationM;
  let maxElevation = points[0].elevationM;
  for (const point of points) {
    minElevation = Math.min(minElevation, point.elevationM);
    maxElevation = Math.max(maxElevation, point.elevationM);
  }
  const elevationPadding = Math.max((maxElevation - minElevation) * 0.1, 20);
  minElevation -= elevationPadding;
  maxElevation += elevationPadding;

  const formatDistance = (distance) => `${distance.toFixed(distance < 10 ? 1 : 0)} km`;
  const formatElevation = (elevation) => `${Math.round(elevation)} m`;
  const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
  const officialProfileController = window.createOfficialClimbProfileController(points);
  const xForDistance = (distance) => {
    const span = Math.max(maxDistance - minDistance, 1);
    return state.plot.left + (distance - minDistance) / span * (state.plot.right - state.plot.left);
  };
  const yForElevation = (elevation) => {
    const span = Math.max(maxElevation - minElevation, 1);
    return state.plot.bottom - (elevation - minElevation) / span * (state.plot.bottom - state.plot.top);
  };
  const nearestPointIndex = (distance) => {
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
  const nearestMapPointIndex = (latitude, longitude) => {
    let nearestIndex = 0;
    let nearestDistance = Infinity;
    for (let index = 0; index < points.length; index++) {
      const point = points[index];
      const distance = L.latLng(point.latitude, point.longitude).distanceTo([latitude, longitude]);
      if (distance < nearestDistance) {
        nearestIndex = index;
        nearestDistance = distance;
      }
    }
    return nearestIndex;
  };
  const boundaryForms = [...document.querySelectorAll("[data-official-climb-form]")];
  let activeBoundary = null;
  const boundaryLabel = (index) => {
    const point = points[index];
    return `${formatDistance(point.distanceKm)} · ${formatElevation(point.elevationM)}`;
  };
  const categoryForScore = (score) => {
    if (score < 35) return "NO";
    if (score < 80) return "Cat 4";
    if (score < 180) return "Cat 3";
    if (score < 250) return "Cat 2";
    if (score < 600) return "Cat 1";
    return "HC";
  };
  const elevationAtDistance = (index, distanceM) => {
    if (index + 1 >= points.length) return points[index].elevationM;
    const startDistanceM = points[index].distanceKm * 1000;
    const endDistanceM = points[index + 1].distanceKm * 1000;
    const fraction = (distanceM - startDistanceM) / (endDistanceM - startDistanceM);
    return points[index].elevationM + fraction * (points[index + 1].elevationM - points[index].elevationM);
  };
  const cotacolForClimb = (startIndex, endIndex) => {
    const startDistanceM = points[startIndex].distanceKm * 1000;
    const lastDistanceM = points[endIndex].distanceKm * 1000;
    if (lastDistanceM <= startDistanceM) return 0;
    let score = 0;
    let pointIndex = startIndex;
    for (let segmentStartM = startDistanceM; segmentStartM < lastDistanceM; segmentStartM += 100) {
      const segmentEndM = Math.min(segmentStartM + 100, lastDistanceM);
      while (pointIndex < endIndex && points[pointIndex + 1].distanceKm * 1000 <= segmentStartM) pointIndex++;
      const startElevation = elevationAtDistance(pointIndex, segmentStartM);
      while (pointIndex < endIndex && points[pointIndex + 1].distanceKm * 1000 < segmentEndM) pointIndex++;
      const endElevation = elevationAtDistance(pointIndex, segmentEndM);
      const slope = (endElevation - startElevation) / (segmentEndM - segmentStartM);
      if (slope > 0) score += (segmentEndM - segmentStartM) / 1000 * (slope * 100) ** 2;
    }
    return score;
  };
  const climbMetrics = (index) => {
    const bounds = state.climbBounds[index];
    if (!bounds || !Number.isInteger(bounds.startIndex) || !Number.isInteger(bounds.endIndex) || bounds.startIndex < 0 || bounds.endIndex >= points.length || bounds.startIndex >= bounds.endIndex) return null;
    const start = points[bounds.startIndex];
    const end = points[bounds.endIndex];
    const distanceKm = end.distanceKm - start.distanceKm;
    const elevationGain = end.elevationM - start.elevationM;
    const slope = distanceKm > 0 ? elevationGain / (distanceKm * 10) : 0;
    const score = distanceKm > 0 ? Math.abs(elevationGain) * elevationGain / (distanceKm * 1000) * 10 : 0;
    return { start, end, distanceKm, elevationGain, slope, score, cotacol: cotacolForClimb(bounds.startIndex, bounds.endIndex), category: categoryForScore(score) };
  };
  const clearBoundarySelection = () => {
    activeBoundary = null;
    for (const form of boundaryForms) {
      for (const button of form.querySelectorAll("[data-boundary-button]")) button.classList.remove("active");
    }
  };
  const updateBoundaryPreview = (form) => {
    const preview = form.querySelector("[data-boundary-preview]");
    const item = form.closest("[data-climb-item]");
    const climbIndex = Number.parseInt(item.dataset.climbIndex, 10);
    const metrics = climbMetrics(climbIndex);
    if (!metrics) {
      preview.textContent = "Choose an end point after the start point.";
      return;
    }
    const summary = item.querySelector("[data-climb-summary]");
    const metricsOutput = item.querySelector("[data-climb-metrics]");
    summary.textContent = `${formatDistance(metrics.start.distanceKm)}–${formatDistance(metrics.end.distanceKm)}`;
    metricsOutput.textContent = `${metrics.category} · ${formatDistance(metrics.distanceKm)} at ${metrics.slope.toFixed(1)}% · Cotacol ${metrics.cotacol.toFixed(1)}`;
    preview.textContent = `Preview: ${formatDistance(metrics.distanceKm)} · ${metrics.elevationGain >= 0 ? "+" : ""}${formatElevation(metrics.elevationGain)} · ${metrics.slope.toFixed(1)}% · Cotacol ${metrics.cotacol.toFixed(1)}`;
  };
  const updateClimbLayer = (index) => {
    const layer = climbLayers[index];
    const metrics = climbMetrics(index);
    if (!layer) return;
    if (!metrics) {
      layer.setLatLngs([]);
      return;
    }
    const bounds = state.climbBounds[index];
    layer.setLatLngs(points.slice(bounds.startIndex, bounds.endIndex + 1).map((point) => [point.latitude, point.longitude]));
    const active = index === state.focusedClimbIndex;
    layer.setStyle({ weight: active ? 9 : 7, opacity: active ? 1 : 0.65 });
  };
  const chooseBoundary = (index) => {
    if (!activeBoundary) return;
    const { form, target } = activeBoundary;
    const item = form.closest("[data-climb-item]");
    const climbIndex = Number.parseInt(item.dataset.climbIndex, 10);
    const input = form.querySelector(`[data-boundary-input="${target}"]`);
    const output = form.querySelector(`[data-boundary-output="${target}"]`);
    input.value = index;
    output.textContent = boundaryLabel(index);
    state.climbBounds[climbIndex][`${target}Index`] = index;
    updateBoundaryPreview(form);
    updateClimbLayer(climbIndex);
    clearBoundarySelection();
    showPoint(index);
  };
  for (const form of boundaryForms) {
    const item = form.closest("[data-climb-item]");
    const climbIndex = Number.parseInt(item.dataset.climbIndex, 10);
    state.climbBounds[climbIndex] = {
      startIndex: Number.parseInt(form.querySelector('[data-boundary-input="start"]').value, 10),
      endIndex: Number.parseInt(form.querySelector('[data-boundary-input="end"]').value, 10),
    };
    updateBoundaryPreview(form);
    for (const button of form.querySelectorAll("[data-boundary-button]")) {
      button.addEventListener("click", () => {
        clearBoundarySelection();
        activeBoundary = { form, target: button.dataset.boundaryButton, source: button.dataset.boundarySource };
        button.classList.add("active");
        const surface = activeBoundary.source === "map" ? "map" : "profile";
        hoverOutput.textContent = `Click the ${surface} to select the ${activeBoundary.target} point.`;
      });
    }
  }

  const draw = () => {
    const rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    const ratio = window.devicePixelRatio || 1;
    canvas.width = Math.floor(rect.width * ratio);
    canvas.height = Math.floor(rect.height * ratio);
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    state.width = rect.width;
    state.height = rect.height;
    state.plot = { left: 52, right: rect.width - 20, top: 24, bottom: rect.height - 34 };
    const plot = state.plot;
    context.clearRect(0, 0, rect.width, rect.height);
    context.fillStyle = colors.plotSurface;
    context.fillRect(0, 0, rect.width, rect.height);

    context.font = "12px system-ui, sans-serif";
    context.textBaseline = "middle";
    for (let step = 0; step <= 4; step++) {
      const fraction = step / 4;
      const y = plot.top + fraction * (plot.bottom - plot.top);
      const elevation = maxElevation - fraction * (maxElevation - minElevation);
      context.strokeStyle = colors.grid;
      context.lineWidth = 1;
      context.beginPath();
      context.moveTo(plot.left, y);
      context.lineTo(plot.right, y);
      context.stroke();
      context.fillStyle = colors.subtle;
      context.textAlign = "right";
      context.fillText(formatElevation(elevation), plot.left - 8, y);
    }

    for (let climbIndex = 0; climbIndex < (profile.climbs || []).length; climbIndex++) {
      const climb = profile.climbs[climbIndex];
      const metrics = climbMetrics(climbIndex);
      if (!metrics) continue;
      const startX = clamp(xForDistance(metrics.start.distanceKm), plot.left, plot.right);
      const endX = clamp(xForDistance(metrics.end.distanceKm), plot.left, plot.right);
      context.fillStyle = climbIndex === state.focusedClimbIndex ? colors.climbFocusFill : colors.accentFill;
      context.fillRect(startX, plot.top, Math.max(endX - startX, 1), plot.bottom - plot.top);
      const label = climb.name || `${metrics.category} ${Math.round(metrics.score)}`;
      context.fillStyle = colors.climbLabel;
      context.textAlign = "center";
      context.textBaseline = "top";
      context.fillText(label, clamp((startX + endX) / 2, plot.left + 24, plot.right - 24), plot.top + 4);
    }

    for (const crossing of profile.crossings || []) {
      const x = xForDistance(crossing.distanceKm);
      context.strokeStyle = colors.crossing;
      context.lineWidth = 1;
      context.setLineDash([4, 3]);
      context.beginPath();
      context.moveTo(x, plot.top);
      context.lineTo(x, plot.bottom);
      context.stroke();
      context.setLineDash([]);
    }

    context.beginPath();
    context.moveTo(xForDistance(points[0].distanceKm), plot.bottom);
    for (const point of points) context.lineTo(xForDistance(point.distanceKm), yForElevation(point.elevationM));
    context.lineTo(xForDistance(points[points.length - 1].distanceKm), plot.bottom);
    context.closePath();
    context.fillStyle = colors.accentFill;
    context.fill();
    context.beginPath();
    for (let i = 0; i < points.length; i++) {
      const point = points[i];
      if (i === 0) context.moveTo(xForDistance(point.distanceKm), yForElevation(point.elevationM));
      else context.lineTo(xForDistance(point.distanceKm), yForElevation(point.elevationM));
    }
    context.strokeStyle = colors.accent;
    context.lineWidth = 2.5;
    context.stroke();
    for (const crossing of profile.crossings || []) {
      const x = xForDistance(crossing.distanceKm);
      const label = `${crossing.name} ${Math.round(crossing.passElevationM)} m`;
      const labelWidth = context.measureText(label).width;
      const labelX = clamp(x, plot.left + labelWidth / 2 + 3, plot.right - labelWidth / 2 - 3);
      const labelY = plot.top - 6;
      context.fillStyle = colors.plotSurfaceOverlay;
      context.fillRect(labelX - labelWidth / 2 - 3, labelY - 15, labelWidth + 6, 16);
      context.fillStyle = colors.crossingLabel;
      context.textAlign = "center";
      context.textBaseline = "bottom";
      context.fillText(label, labelX, labelY);
    }

    context.fillStyle = colors.subtle;
    context.textAlign = "center";
    context.textBaseline = "top";
    for (let step = 0; step <= 4; step++) {
      const distance = minDistance + step / 4 * (maxDistance - minDistance);
      context.fillText(formatDistance(distance), xForDistance(distance), plot.bottom + 10);
    }

    if (state.hoveredIndex >= 0) {
      const point = points[state.hoveredIndex];
      const x = xForDistance(point.distanceKm);
      const y = yForElevation(point.elevationM);
      context.strokeStyle = colors.hoverLine;
      context.lineWidth = 1;
      context.setLineDash([3, 3]);
      context.beginPath();
      context.moveTo(x, plot.top);
      context.lineTo(x, plot.bottom);
      context.stroke();
      context.setLineDash([]);
      context.fillStyle = colors.forest;
      context.beginPath();
      context.arc(x, y, 5, 0, 2 * Math.PI);
      context.fill();
    }
  };

  const clearHover = () => {
    state.hoveredIndex = -1;
    routeCursor.setStyle({ opacity: 0, fillOpacity: 0 });
    hoverOutput.textContent = "Hover or focus the profile to inspect elevation.";
    draw();
  };
  const showPoint = (index) => {
    state.hoveredIndex = clamp(index, 0, points.length - 1);
    const point = points[state.hoveredIndex];
    routeCursor.setLatLng([point.latitude, point.longitude]);
    routeCursor.setStyle({ opacity: 1, fillOpacity: 0.9 });
    hoverOutput.textContent = `${formatDistance(point.distanceKm)} · ${formatElevation(point.elevationM)}`;
    draw();
  };
  const zoomToClimb = (index) => {
    const climbBounds = state.climbBounds[index];
    if (!climbBounds || !Number.isInteger(climbBounds.startIndex) || !Number.isInteger(climbBounds.endIndex)) return;
    const climbPoints = points.slice(climbBounds.startIndex, climbBounds.endIndex + 1);
    const mapBounds = L.latLngBounds(climbPoints.map((point) => [point.latitude, point.longitude]));
    if (mapBounds.isValid()) map.fitBounds(mapBounds, { padding: [32, 32], maxZoom: 15 });
  };
  const updateClimbFocus = (index, zoom) => {
    if (climbItems.length === 0) return;
    state.focusedClimbItemIndex = clamp(index, 0, climbItems.length - 1);
    state.focusedClimbIndex = climbItemIndices[state.focusedClimbItemIndex];
    for (let itemIndex = 0; itemIndex < climbItems.length; itemIndex++) {
      const active = itemIndex === state.focusedClimbItemIndex;
      climbItems[itemIndex].hidden = !active;
      climbItems[itemIndex].setAttribute("aria-hidden", String(!active));
      climbItems[itemIndex].classList.toggle("focused", active);
      const climbIndex = climbItemIndices[itemIndex];
      if (climbLayers[climbIndex]) {
        updateClimbLayer(climbIndex);
      }
    }
    if (climbPosition) climbPosition.textContent = `Climb ${state.focusedClimbItemIndex + 1} of ${climbItems.length}`;
    if (previousClimbButton) previousClimbButton.disabled = state.focusedClimbItemIndex === 0;
    if (nextClimbButton) nextClimbButton.disabled = state.focusedClimbItemIndex === climbItems.length - 1;
    clearBoundarySelection();
    draw();
    if (zoom) zoomToClimb(state.focusedClimbIndex);
  };
  if (previousClimbButton) previousClimbButton.addEventListener("click", () => updateClimbFocus(state.focusedClimbItemIndex - 1, true));
  if (nextClimbButton) nextClimbButton.addEventListener("click", () => updateClimbFocus(state.focusedClimbItemIndex + 1, true));
  canvas.addEventListener("pointermove", (event) => {
    if (!state.plot) return;
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    if (x < state.plot.left || x > state.plot.right) {
      clearHover();
      return;
    }
    const distance = minDistance + (x - state.plot.left) / (state.plot.right - state.plot.left) * (maxDistance - minDistance);
    showPoint(nearestPointIndex(distance));
  });
  canvas.addEventListener("pointerleave", clearHover);
  canvas.addEventListener("pointercancel", clearHover);
  canvas.addEventListener("click", (event) => {
    if (!activeBoundary || activeBoundary.source !== "profile" || !state.plot) return;
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    if (x < state.plot.left || x > state.plot.right) return;
    const distance = minDistance + (x - state.plot.left) / (state.plot.right - state.plot.left) * (maxDistance - minDistance);
    chooseBoundary(nearestPointIndex(distance));
  });
  map.on("click", (event) => {
    if (!activeBoundary || activeBoundary.source !== "map") return;
    chooseBoundary(nearestMapPointIndex(event.latlng.lat, event.latlng.lng));
  });
  canvas.addEventListener("keydown", (event) => {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
    event.preventDefault();
    const direction = event.key === "ArrowRight" ? 1 : -1;
    const index = state.hoveredIndex < 0 ? (direction > 0 ? 0 : points.length - 1) : state.hoveredIndex + direction;
    showPoint(index);
  });
  window.addEventListener("resize", () => {
    draw();
    officialProfileController.redrawOpen();
  });
  if (climbItems.length > 0) updateClimbFocus(0, false);
  else draw();
})();
