(() => {
  const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
  const profileStepSizesM = [100, 200, 500, 1000];
  const displayStepForLength = (lengthM) => profileStepSizesM.find((stepM) => Math.ceil(lengthM / stepM) <= 30) || profileStepSizesM[profileStepSizesM.length - 1];
  const profileBandForSlope = (slopePercent) => {
    if (slopePercent < 0) return "downhill";
    if (slopePercent < 3) return "0-3";
    if (slopePercent < 6) return "3-6";
    if (slopePercent < 9) return "6-9";
    if (slopePercent < 12) return "9-12";
    return "12-plus";
  };
  const officialProfileSections = (points, startIndex, endIndex, stepM) => {
    const startDistanceM = points[startIndex].distanceKm * 1000;
    const endDistanceM = points[endIndex].distanceKm * 1000;
    const sections = [];
    let pointIndex = startIndex;
    const elevationAtDistance = (distanceM) => {
      while (pointIndex < endIndex - 1 && points[pointIndex + 1].distanceKm * 1000 < distanceM) pointIndex++;
      const nextIndex = Math.min(pointIndex + 1, endIndex);
      const first = points[pointIndex];
      const next = points[nextIndex];
      const distanceSpan = next.distanceKm * 1000 - first.distanceKm * 1000;
      if (distanceSpan <= 0) return first.elevationM;
      const fraction = (distanceM - first.distanceKm * 1000) / distanceSpan;
      return first.elevationM + clamp(fraction, 0, 1) * (next.elevationM - first.elevationM);
    };
    for (let sectionStartM = startDistanceM; sectionStartM < endDistanceM; sectionStartM += stepM) {
      const sectionEndM = Math.min(sectionStartM + stepM, endDistanceM);
      const startElevation = elevationAtDistance(sectionStartM);
      const endElevation = elevationAtDistance(sectionEndM);
      const slopePercent = (endElevation - startElevation) / (sectionEndM - sectionStartM) * 100;
      sections.push({
        startDistanceKm: sectionStartM / 1000,
        endDistanceKm: sectionEndM / 1000,
        startElevation,
        endElevation,
        slopePercent,
        band: profileBandForSlope(slopePercent),
      });
    }
    return sections;
  };
  const api = { displayStepForLength, profileBandForSlope, officialProfileSections };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  else window.OfficialClimbProfileLogic = api;
})();
