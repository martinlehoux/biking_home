(() => {
  const createOfficialClimbProfileController = (points) => {
    const { displayStepForLength, officialProfileSections } = window.OfficialClimbProfileLogic;
    const colorProbe = document.createElement("span");
    colorProbe.hidden = true;
    document.body.append(colorProbe);
    const resolveColor = (name) => {
      colorProbe.style.color = `var(${name})`;
      return getComputedStyle(colorProbe).color;
    };
    const colors = {
      downhill: resolveColor("--color-profile-downhill"),
      "0-3": resolveColor("--color-profile-0-3"),
      "3-6": resolveColor("--color-profile-3-6"),
      "6-9": resolveColor("--color-profile-6-9"),
      "9-12": resolveColor("--color-profile-9-12"),
      "12-plus": resolveColor("--color-profile-12-plus"),
      plotSurface: resolveColor("--color-plot-surface"),
      grid: resolveColor("--color-plot-grid"),
      subtle: resolveColor("--color-subtle"),
      accent: resolveColor("--color-accent"),
    };
    colorProbe.remove();
    const formatDistance = (distance) => `${distance.toFixed(distance < 10 ? 1 : 0)} km`;
    const drawOfficialProfile = (profileCanvas) => {
      const startIndex = Number.parseInt(profileCanvas.dataset.profileStart, 10);
      const endIndex = Number.parseInt(profileCanvas.dataset.profileEnd, 10);
      if (!Number.isInteger(startIndex) || !Number.isInteger(endIndex) || startIndex < 0 || endIndex >= points.length || startIndex >= endIndex) return;
      const rect = profileCanvas.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) return;
      const context = profileCanvas.getContext("2d");
      if (!context) return;
      const ratio = window.devicePixelRatio || 1;
      profileCanvas.width = Math.floor(rect.width * ratio);
      profileCanvas.height = Math.floor(rect.height * ratio);
      context.setTransform(ratio, 0, 0, ratio, 0, 0);
      const plot = { left: 10, right: rect.width - 10, top: 16, bottom: rect.height - 24 };
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
      const xForDistance = (distanceKm) => plot.left + (distanceKm - minDistance) / distanceSpan * (plot.right - plot.left);
      const yForElevation = (elevationM) => plot.bottom - (elevationM - minElevation) / elevationSpan * (plot.bottom - plot.top);
      context.clearRect(0, 0, rect.width, rect.height);
      context.fillStyle = colors.plotSurface;
      context.fillRect(0, 0, rect.width, rect.height);
      context.strokeStyle = colors.grid;
      context.lineWidth = 1;
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
        const labelY = Math.max(plot.top + 10, Math.min(yForElevation(section.startElevation), yForElevation(section.endElevation)) - 4);
        context.fillText(label, (startX + endX) / 2, labelY);
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
      context.font = "11px system-ui, sans-serif";
      context.fillStyle = colors.subtle;
      context.textBaseline = "top";
      context.textAlign = "left";
      context.fillText(formatDistance(0), plot.left, rect.height - 16);
      context.textAlign = "right";
      context.fillText(formatDistance(maxDistance - minDistance), plot.right, rect.height - 16);
    };
    const cards = [...document.querySelectorAll("[data-official-climb-card]")];
    for (const card of cards) {
      card.addEventListener("toggle", () => {
        if (!card.open) return;
        for (const other of cards) {
          if (other !== card) other.open = false;
        }
        const profileCanvas = card.querySelector("[data-official-profile]");
        if (profileCanvas) drawOfficialProfile(profileCanvas);
      });
    }
    return {
      redrawOpen: () => {
        for (const profileCanvas of document.querySelectorAll("[data-official-profile]")) {
          const card = profileCanvas.closest("[data-official-climb-card]");
          if (card?.open) drawOfficialProfile(profileCanvas);
        }
      },
    };
  };

  window.createOfficialClimbProfileController = createOfficialClimbProfileController;
})();
