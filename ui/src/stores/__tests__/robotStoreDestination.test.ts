import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { Track } from '@/types/api'
import { useRobotStore } from '../robotStore'
import { useUIStore } from '../uiStore'

function track(id: number, tagId: number | undefined, state: Track['state'] = 'confirmed'): Track {
    return { id, bbox: [0, 0, 10, 10], confidence: 1, tag_id: tagId, state, history: [] }
}

function setup() {
    setActivePinia(createPinia())
    const robot = useRobotStore()
    const ui = useUIStore()
    const toast = vi.spyOn(ui, 'showToast')
    const log = vi.spyOn(ui, 'addLogEntry')
    return { robot, ui, toast, log }
}

function jsonResponse(status: number, body: unknown): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
    })
}

describe('selection follows the robot across track id changes', () => {
    it('remembers the tag id when a robot is selected', () => {
        const { robot } = setup()
        robot.setTracks([track(7, 1)])
        robot.selectTrack(7)
        expect(robot.selectedTagId).toBe(1)
    })

    it('remaps to the new track id when the tracker reassigns it', () => {
        const { robot } = setup()
        robot.setTracks([track(7, 1), track(8, 2)])
        robot.selectTrack(7)

        robot.setTracks([track(21, 1), track(8, 2)])

        expect(robot.selectedTrackId).toBe(21)
        expect(robot.selectedTagId).toBe(1)
    })

    it('leaves destination mode unchanged when remapping', () => {
        const { robot } = setup()
        robot.setTracks([track(7, 1)])
        robot.selectTrack(7)
        robot.cancelDestinationMode()

        robot.setTracks([track(9, 1)])
        expect(robot.selectedTrackId).toBe(9)
        expect(robot.destinationMode).toBe(false)

        robot.selectTrack(9)
        robot.setTracks([track(12, 1)])
        expect(robot.destinationMode).toBe(true)
    })

    it('remaps through the incremental single-track update path too', () => {
        const { robot } = setup()
        robot.setTracks([track(7, 1)])
        robot.selectTrack(7)
        robot.setTracks([])

        robot.handleWebSocketMessage({ type: 'track', track: track(30, 1) })
        expect(robot.selectedTrackId).toBe(30)
    })

    it('keeps the selection while the tag is untracked and remaps when it reappears', () => {
        const { robot } = setup()
        robot.setTracks([track(7, 1)])
        robot.selectTrack(7)

        robot.setTracks([track(8, 2)])
        expect(robot.selectedTrackId).toBe(7)
        expect(robot.selectedTagId).toBe(1)

        robot.setTracks([track(8, 2), track(40, 1)])
        expect(robot.selectedTrackId).toBe(40)
    })

    it('does not remap onto an unconfirmed track', () => {
        const { robot } = setup()
        robot.setTracks([track(7, 1)])
        robot.selectTrack(7)
        robot.setTracks([track(41, 1, 'pending')])
        expect(robot.selectedTrackId).toBe(7)
    })

    it('does not touch a selection whose track still exists', () => {
        const { robot } = setup()
        robot.setTracks([track(7, 1), track(8, 1)])
        robot.selectTrack(7)
        robot.setTracks([track(7, 1), track(8, 1)])
        expect(robot.selectedTrackId).toBe(7)
    })

    it('clearSelection forgets the tag', () => {
        const { robot } = setup()
        robot.setTracks([track(7, 1)])
        robot.selectTrack(7)
        robot.clearSelection()
        expect(robot.selectedTagId).toBeNull()
        robot.setTracks([track(9, 1)])
        expect(robot.selectedTrackId).toBeNull()
    })
})

describe('confirmDestination fails loudly', () => {
    beforeEach(() => {
        vi.stubGlobal('fetch', vi.fn())
    })

    afterEach(() => {
        vi.unstubAllGlobals()
        vi.restoreAllMocks()
    })

    function expectFailure(
        ctx: ReturnType<typeof setup>,
        message: string | RegExp,
        options: { keepsMode?: boolean } = {}
    ): void {
        expect(ctx.toast).toHaveBeenCalledTimes(1)
        const [text, type] = ctx.toast.mock.calls[0] ?? []
        expect(type).toBe('error')
        if (typeof message === 'string') expect(text).toBe(message)
        else expect(text).toMatch(message)
        expect(ctx.log).toHaveBeenCalledWith('error', text)
        if (options.keepsMode) expect(ctx.robot.destinationMode).toBe(true)
    }

    it('says to click the robot first when nothing is selected', async () => {
        const ctx = setup()
        expect(await ctx.robot.confirmDestination(10, 10)).toBe(false)
        expectFailure(ctx, 'Click the robot first, then click where it should go')
        expect(fetch).not.toHaveBeenCalled()
    })

    it('says the robot is not visible when its tag is not tracked', async () => {
        const ctx = setup()
        ctx.robot.setTracks([track(7, 1)])
        ctx.robot.selectTrack(7)
        ctx.robot.setTracks([track(8, 2)])

        expect(await ctx.robot.confirmDestination(10, 10)).toBe(false)
        expectFailure(
            ctx,
            'Robot 1 is not visible right now; wait for it to be detected, then try again',
            { keepsMode: true }
        )
        expect(fetch).not.toHaveBeenCalled()
    })

    it('recovers a stale track id by tag and sends the destination', async () => {
        const ctx = setup()
        ctx.robot.setTracks([track(7, 1)])
        ctx.robot.selectTrack(7)
        // The list changed under the selection without going through the remap path.
        ctx.robot.tracks.splice(0, ctx.robot.tracks.length, track(55, 1))
        vi.mocked(fetch).mockResolvedValue(jsonResponse(200, { status: 'ok' }))

        expect(await ctx.robot.confirmDestination(10, 10)).toBe(true)
        const body = JSON.parse(String(vi.mocked(fetch).mock.calls[0]?.[1]?.body)) as {
            robot_id: number
        }
        expect(body.robot_id).toBe(1)
        expect(ctx.toast).not.toHaveBeenCalled()
    })

    it('explains a track that has no AprilTag', async () => {
        const ctx = setup()
        ctx.robot.setTracks([track(7, undefined)])
        ctx.robot.selectTrack(7)
        expect(await ctx.robot.confirmDestination(10, 10)).toBe(false)
        expectFailure(ctx, 'That object has no AprilTag, so it cannot be sent anywhere')
    })

    it("shows the server's own rejection text", async () => {
        const ctx = setup()
        ctx.robot.setTracks([track(7, 1)])
        ctx.robot.selectTrack(7)
        vi.mocked(fetch).mockResolvedValue(
            jsonResponse(400, { error: 'Destination overlaps with obstacle', obstacle: 'Chair' })
        )

        expect(await ctx.robot.confirmDestination(10, 10)).toBe(false)
        expectFailure(ctx, 'Destination overlaps with obstacle', { keepsMode: true })
    })

    it('falls back to a generic message for a non-JSON rejection', async () => {
        const ctx = setup()
        ctx.robot.setTracks([track(7, 1)])
        ctx.robot.selectTrack(7)
        vi.mocked(fetch).mockResolvedValue(new Response('boom', { status: 502 }))

        expect(await ctx.robot.confirmDestination(10, 10)).toBe(false)
        expectFailure(ctx, /rejected the destination \(HTTP 502\)/)
    })

    it('reports a network failure', async () => {
        const ctx = setup()
        vi.spyOn(console, 'error').mockImplementation(() => undefined)
        ctx.robot.setTracks([track(7, 1)])
        ctx.robot.selectTrack(7)
        vi.mocked(fetch).mockRejectedValue(new TypeError('Failed to fetch'))

        expect(await ctx.robot.confirmDestination(10, 10)).toBe(false)
        expectFailure(ctx, 'Could not reach the tracker service', { keepsMode: true })
    })

    it('a successful destination logs success and shows no error', async () => {
        const ctx = setup()
        ctx.robot.setTracks([track(7, 1)])
        ctx.robot.selectTrack(7)
        vi.mocked(fetch).mockResolvedValue(jsonResponse(200, { status: 'ok' }))

        expect(await ctx.robot.confirmDestination(10, 10)).toBe(true)
        expect(ctx.toast).not.toHaveBeenCalled()
        expect(ctx.log).toHaveBeenCalledWith('success', 'Destination set for robot 1')
        expect(ctx.robot.destinationMode).toBe(false)
    })
})

describe('reconnecting to a backend that may have restarted', () => {
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('resetTransientState drops the goal, paths, motion, tracks and selection', () => {
        const { robot } = setup()
        robot.setTracks([track(7, 1)])
        robot.selectTrack(7)
        robot.setDestination({ id: 'd', robot_id: 1, x: 5, y: 6 })
        robot.handleWebSocketMessage({
            type: 'motion',
            motion: { code: 'moving', text: 'Driving', severity: 'ok' },
        } as never)

        robot.resetTransientState()

        expect(robot.destination).toBeNull()
        expect(robot.motion).toBeNull()
        expect(robot.tracks).toEqual([])
        expect(robot.paths).toEqual([])
        expect(robot.selectedTrackId).toBeNull()
        expect(robot.destinationMode).toBe(false)
    })

    it('fetchDestination restores a goal the server still has', async () => {
        const { robot } = setup()
        vi.stubGlobal(
            'fetch',
            vi.fn().mockResolvedValue(jsonResponse(200, { robot_id: 2, x: 10, y: 20, valid: true }))
        )
        await robot.fetchDestination()
        expect(robot.destination).toMatchObject({ robot_id: 2, x: 10, y: 20 })
    })

    it('fetchDestination clears a goal the server does not have', async () => {
        const { robot } = setup()
        robot.setDestination({ id: 'd', robot_id: 1, x: 5, y: 6 })
        vi.stubGlobal(
            'fetch',
            vi.fn().mockResolvedValue(jsonResponse(200, { robot_id: 0, x: 0, y: 0, valid: false }))
        )
        await robot.fetchDestination()
        expect(robot.destination).toBeNull()
    })
})
