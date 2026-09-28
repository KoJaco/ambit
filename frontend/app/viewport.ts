/** Minimum size for the architect canvas (sidebar, inspector, and graph). */
export const MIN_VIEWPORT_WIDTH = 900;
export const MIN_VIEWPORT_HEIGHT = 600;

export const desktopViewportQuery = `(min-width: ${MIN_VIEWPORT_WIDTH}px) and (min-height: ${MIN_VIEWPORT_HEIGHT}px)`;

export function isDesktopViewport(
    width = window.innerWidth,
    height = window.innerHeight,
): boolean {
    return width >= MIN_VIEWPORT_WIDTH && height >= MIN_VIEWPORT_HEIGHT;
}
