import * as React from "react"

/**
 * Runs `reset` on the render where `open` flips from false to true, so a dialog
 * that is reopened never flashes what was left in it last time.
 *
 * Deliberately a render-phase adjustment rather than an effect: an effect would
 * paint the stale values for a frame before clearing them.
 */
function useResetOnOpen(open: boolean, reset: () => void) {
  const [wasOpen, setWasOpen] = React.useState(open)

  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) {
      reset()
    }
  }
}

export { useResetOnOpen }
