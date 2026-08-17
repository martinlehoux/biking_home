export type ProfileBand = "downhill" | "0-3" | "3-6" | "6-9" | "9-plus";
export type BoundaryTarget = "start" | "end";
export type BoundarySource = "profile" | "map";

export interface ProfilePoint {
  distanceKm: number;
  elevationM: number;
}

export interface RideProfilePoint extends ProfilePoint {
  latitude: number;
  longitude: number;
}

export interface RideProfileClimb {
  startKm: number;
  endKm: number;
  topKm: number;
  topElevationM: number;
  name: string;
  category: string;
  distanceKm: number;
  slopePercent: number;
  cotacol: number;
  officialClimbId?: number;
  officialName?: string;
  startIndex: number;
  endIndex: number;
}

export interface RideProfileCrossing {
  distanceKm: number;
  passElevationM: number;
  rideElevationM: number;
  distanceToM: number;
  elevationDiffM: number;
  name: string;
}

export interface RideProfile {
  points: RideProfilePoint[];
  climbs: RideProfileClimb[];
  crossings: RideProfileCrossing[];
}

export interface RideRoute {
  type: "FeatureCollection";
  features: RideRouteFeature[];
}

export interface RideRouteFeature {
  type: "Feature";
  geometry: {
    type: "LineString";
    coordinates: [number, number][];
  };
  properties?: Record<string, unknown>;
}

export interface RideMapPass {
  name: string;
  elevationM: number;
  latitude: number;
  longitude: number;
}

export interface ClimbBounds {
  startIndex: number;
  endIndex: number;
}

export interface ClimbMetrics {
  start: ProfilePoint;
  end: ProfilePoint;
  distanceKm: number;
  elevationGain: number;
  slope: number;
  cotacol: number;
  category: string;
}

export interface OfficialProfileColors {
  downhill: string;
  "0-3": string;
  "3-6": string;
  "6-9": string;
  "9-plus": string;
  plotSurface: string;
  grid: string;
  xGrid: string;
  yGrid: string;
  subtle: string;
  accent: string;
}

export interface RideProfileColors {
  subtle: string;
  plotSurface: string;
  grid: string;
  forest: string;
  plotSurfaceOverlay: string;
  accentFill: string;
  profileFill: string;
  profileLine: string;
  climbLabel: string;
  crossing: string;
  crossingLabel: string;
  hoverLine: string;
  climbFocusFill: string;
}

export interface RideMapColors {
  accent: string;
  forest: string;
  climbRoute: string;
}

export interface SyncProgress {
  total: number;
  completed: number;
  imported: number;
  skipped: number;
}

export interface SyncErrorEvent {
  message: string;
  progress?: SyncProgress;
}
