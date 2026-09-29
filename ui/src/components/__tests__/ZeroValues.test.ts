import { describe, it, expect, afterEach } from 'vitest'
import { createApp, nextTick, type App } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import TelemetryPanel from '../TelemetryPanel.vue'
import TrackList from '../TrackList.vue'
import { useRobotStore } from '@/stores/robotStore'
import type { Track } from '@/types/api'

// 0 is a real value for both: heading 0 rad faces +x, and tag id 0 is a valid tag.
// Both used to be hidden by falsy checks.
function track(overrides: Partial<Track>): Track {
    return {
        id: 1,
        bbox: [0, 0, 50, 50],
        confidence: 1,
        state: 'confirmed',
        history: [],
        ...overrides,
    } as Track
}

describe('zero values are shown, not treated as missing', () => {
    let app: App | undefined
    let root: HTMLElement | undefined

    afterEach(() => {
        app?.unmount()
        root?.remove()
    })

    function mount(component: object): HTMLElement {
        const pinia = createPinia()
        setActivePinia(pinia)
        root = document.createElement('div')
        document.body.appendChild(root)
        app = createApp(component)
        app.use(pinia)
        app.mount(root)
        return root
    }

    // The Heading row's own value; CompassHeading renders heading text too, so a
    // whole-panel text search would match it and prove nothing.
    const headingValue = (el: HTMLElement) =>
        el.querySelector('.telemetry-value')?.textContent?.trim()

    it('shows a heading of exactly 0 as 0.0°', async () => {
        const el = mount(TelemetryPanel)
        const robot = useRobotStore()
        robot.setTracks([track({ id: 5, tag_id: 1, heading: 0 })])
        robot.selectTrack(5)
        await nextTick()

        expect(headingValue(el)).toBe('0.0°')
    })

    it('shows -- when there is no heading at all', async () => {
        const el = mount(TelemetryPanel)
        const robot = useRobotStore()
        robot.setTracks([track({ id: 5, tag_id: 1 })])
        robot.selectTrack(5)
        await nextTick()

        expect(headingValue(el)).toBe('--')
    })

    it('shows the tag badge for tag id 0', async () => {
        const el = mount(TrackList)
        useRobotStore().setTracks([track({ id: 3, tag_id: 0, configured: true })])
        await nextTick()

        expect(el.querySelector('.track-tag')?.textContent).toContain('Tag 0')
    })
})
