// Canvas color/font theme constants — matches CSS custom properties
export const THEME = {
    // Footprint colors
    footprintFill: 'rgba(0, 217, 255, 0.15)',
    footprintStroke: '#00d9ff',
    footprintSelectedFill: 'rgba(224, 230, 237, 0.25)',
    footprintSelectedStroke: '#e0e6ed',

    // Temporary obstacles (foreground detector)
    tempFill: 'rgba(255, 145, 0, 0.28)',
    tempStroke: '#ff9100',
    tempIdleFill: 'rgba(255, 145, 0, 0.08)',
    tempIdleStroke: 'rgba(255, 145, 0, 0.7)',
    tempChipBg: 'rgba(10, 14, 20, 0.85)',
    tempChipText: '#ffab00',

    // Destination cursor
    destValidFill: 'rgba(0, 230, 118, 0.3)',
    destValidStroke: '#00e676',
    destInvalidFill: 'rgba(255, 61, 0, 0.3)',
    destInvalidStroke: '#ff3d00',

    // Destination marker (goal)
    goalFill: 'rgba(0, 145, 234, 0.3)',
    goalStroke: '#0091ea',

    // Drawing box (obstacle drawing)
    drawingStroke: '#ffab00',
    drawingFill: 'rgba(255, 171, 0, 0.2)',

    // Movement indicator
    headingChevron: '#00d9ff',
    headingChevronGlow: 'rgba(0, 217, 255, 0.4)',
    thrustForward: '#00d9ff',
    thrustReverse: '#ffab00',
    rotationArc: '#00d9ff',

    // Calibration tag
    calibTagStroke: 'rgba(224, 230, 237, 0.5)',
    calibTagLabel: 'rgba(224, 230, 237, 0.7)',
    calibGuideStroke: 'rgba(255, 255, 255, 0.8)',
    calibOkStroke: '#00e676',
    calibOkFill: 'rgba(0, 230, 118, 0.25)',
    calibWarnStroke: '#ffab00',
    calibWarnFill: 'rgba(255, 171, 0, 0.25)',
    calibBlockStroke: '#ff3d00',
    calibBlockFill: 'rgba(255, 61, 0, 0.25)',

    // Invalid flash
    flashOuter: 'rgba(255, 61, 0, 0.8)',
    flashInner: 'rgba(255, 61, 0, 0.4)',

    // Label backgrounds / text
    labelBg: '#0a0e14',
    textPrimary: '#e0e6ed',

    // Fonts
    fontLabel: "bold 11px 'Inter', sans-serif",
    fontSmall: "10px 'JetBrains Mono', monospace",
    fontDest: "bold 12px 'Inter', sans-serif",
    fontCalibSmall: "11px 'Inter', sans-serif",
    fontCalibLarge: "bold 14px 'Inter', sans-serif",
} as const
