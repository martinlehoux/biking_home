export const requireElement = <T extends Element>(element: T | null, description: string): T => {
  if (!element) throw new Error(`Missing ${description} element`);
  return element;
};
