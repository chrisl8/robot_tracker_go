const originalWarn = console.warn
console.warn = (...args: unknown[]) => {
    const message = args.join(' ')
    if (
        message.includes('onMounted is called when there is no active component instance') ||
        message.includes('onUnmounted is called when there is no active component instance')
    ) {
        return
    }
    originalWarn.apply(console, args)
}
