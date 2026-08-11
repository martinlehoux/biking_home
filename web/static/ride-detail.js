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
  const state = { hoveredIndex: -1, plot: null, width: 0, height: 0 };
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

    for (const climb of profile.climbs || []) {
      const startX = clamp(xForDistance(climb.startKm), plot.left, plot.right);
      const endX = clamp(xForDistance(climb.endKm), plot.left, plot.right);
      context.fillStyle = colors.accentFill;
      context.fillRect(startX, plot.top, Math.max(endX - startX, 1), plot.bottom - plot.top);
      const label = climb.name || `${climb.category} ${Math.round(climb.score)}`;
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
  canvas.addEventListener("keydown", (event) => {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
    event.preventDefault();
    const direction = event.key === "ArrowRight" ? 1 : -1;
    const index = state.hoveredIndex < 0 ? (direction > 0 ? 0 : points.length - 1) : state.hoveredIndex + direction;
    showPoint(index);
  });
  window.addEventListener("resize", draw);
  draw();
})();
